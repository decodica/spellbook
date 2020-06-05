package content

import (
	"context"
	"decodica.com/spellbook"
	"decodica.com/spellbook/identity"
	"errors"
	"fmt"
	"github.com/decodica/model/v2"
	"google.golang.org/appengine/log"
	"reflect"
	"sort"
	"time"
)

func NewAttachmentController() *spellbook.RestController {
	return NewAttachmentControllerWithKey("")
}

func NewAttachmentControllerWithKey(key string) *spellbook.RestController {
	man := AttachmentManager{}
	handler := spellbook.BaseRestHandler{Manager: man}
	c := spellbook.NewRestController(handler)
	c.Key = key
	return c
}

type AttachmentManager struct{}

func (manager AttachmentManager) NewResource(ctx context.Context) (spellbook.Resource, error) {
	return &Attachment{}, nil
}

func (manager AttachmentManager) FromId(ctx context.Context, strId string) (spellbook.Resource, error) {

	if current := spellbook.IdentityFromContext(ctx); current == nil || (!current.HasPermission(spellbook.PermissionReadContent) && !current.HasPermission(spellbook.PermissionReadMedia)) {
		var p spellbook.Permission
		p = spellbook.PermissionReadContent
		if !current.HasPermission(spellbook.PermissionReadMedia) {
			p = spellbook.PermissionReadMedia
		}
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(p))
	}

	att := Attachment{}
	if err := model.FromEncodedKey(ctx, &att, strId); err != nil {
		log.Errorf(ctx, "could not retrieve attachment %s: %s", strId, err.Error())
		return nil, err
	}

	return &att, nil
}

func (manager AttachmentManager) ListOf(ctx context.Context, opts spellbook.ListOptions) ([]spellbook.Resource, error) {
	if current := spellbook.IdentityFromContext(ctx); current == nil || (!current.HasPermission(spellbook.PermissionReadContent) && !current.HasPermission(spellbook.PermissionReadMedia)) {
		var p spellbook.Permission
		p = spellbook.PermissionReadContent
		if !current.HasPermission(spellbook.PermissionReadMedia) {
			p = spellbook.PermissionReadMedia
		}
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(p))
	}

	var attachments []*Attachment
	q := model.NewQuery(&Attachment{})
	q = q.OffsetBy(opts.Page * opts.Size)

	if opts.Order != "" {
		dir := model.ASC
		if opts.Descending {
			dir = model.DESC
		}
		q = q.OrderBy(opts.Order, dir)
	}

	for _, filter := range opts.Filters {
		if filter.Field != "" {
			q = q.WithField(filter.Field+" =", filter.Value)
		}
	}

	// get one more so we know if we are done
	q = q.Limit(opts.Size + 1)
	err := q.GetMulti(ctx, &attachments)
	if err != nil {
		return nil, err
	}

	resources := make([]spellbook.Resource, len(attachments))
	for i := range attachments {
		resources[i] = attachments[i]
	}

	return resources, nil
}

func (manager AttachmentManager) ListOfProperties(ctx context.Context, opts spellbook.ListOptions) ([]string, error) {
	if current := spellbook.IdentityFromContext(ctx); current == nil || (!current.HasPermission(spellbook.PermissionReadContent) && !current.HasPermission(spellbook.PermissionReadMedia)) {
		var p spellbook.Permission
		p = spellbook.PermissionReadContent
		if !current.HasPermission(spellbook.PermissionReadMedia) {
			p = spellbook.PermissionReadMedia
		}
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(p))
	}

	a := []string{"Group", "ParentKey"} // list property accepted
	name := opts.Property

	i := sort.Search(len(a), func(i int) bool { return name <= a[i] })
	if i < len(a) && a[i] == name {
		// found
	} else {
		return nil, errors.New("no property found")
	}

	var conts []*Attachment
	q := model.NewQuery(&Attachment{})
	q = q.OffsetBy(opts.Page * opts.Size)

	if opts.Order != "" {
		dir := model.ASC
		if opts.Descending {
			dir = model.DESC
		}
		q = q.OrderBy(opts.Order, dir)
	}

	for _, filter := range opts.Filters {
		if filter.Field != "" {
			q = q.WithField(filter.Field+" =", filter.Value)
		}
	}

	q = q.Distinct(name)
	q = q.Limit(opts.Size + 1)
	err := q.GetAll(ctx, &conts)
	if err != nil {
		log.Errorf(ctx, "Error retrieving result: %+v", err)
		return nil, err
	}
	var result []string
	for _, c := range conts {
		value := reflect.ValueOf(c).Elem().FieldByName(name).String()
		if len(value) > 0 {
			result = append(result, value)
		}
	}
	return result, nil
}

func (manager AttachmentManager) Create(ctx context.Context, res spellbook.Resource, bundle []byte) error {
	current := spellbook.IdentityFromContext(ctx)
	if current := spellbook.IdentityFromContext(ctx); current == nil || (!current.HasPermission(spellbook.PermissionWriteContent) && !current.HasPermission(spellbook.PermissionWriteMedia)) {
		var p spellbook.Permission
		p = spellbook.PermissionWriteContent
		if !current.HasPermission(spellbook.PermissionWriteMedia) {
			p = spellbook.PermissionWriteMedia
		}
		return spellbook.NewPermissionError(spellbook.PermissionName(p))
	}

	attachment := res.(*Attachment)

	// attachment parent is required.
	// if not attachment is to be specified the default value must be used
	if attachment.ParentKey == "" {
		msg := fmt.Sprintf("attachment parent can't be empty. Use %s as a parent for global attachments", AttachmentGlobalParent)
		return spellbook.NewFieldError("parent", errors.New(msg))
	}

	if attachment.ResourceThumbUrl == "" {
		log.Infof(ctx, "No thumbnail provided for attachment %s, the image url will be used", attachment.Name)
		attachment.ResourceThumbUrl = attachment.ResourceUrl
	}

	// test the attachment parent type
	if sa := SupportedAttachmentsFromContext(ctx); sa != nil {
		if !sa.IsSupported(attachment) {
			msg := fmt.Sprintf("unsupported parent type %q for attachment", attachment.ParentType)
			return spellbook.NewFieldError("parentType", errors.New(msg))
		}
	}

	attachment.Created = time.Now().UTC()
	attachment.Uploader = current.(identity.User).Username()

	err := model.Create(ctx, attachment)
	if err != nil {
		log.Errorf(ctx, "error creating attachment %s: %s", attachment.Name, err)
		return err
	}

	return nil
}

func (manager AttachmentManager) Update(ctx context.Context, res spellbook.Resource, bundle []byte) error {
	if current := spellbook.IdentityFromContext(ctx); current == nil || (!current.HasPermission(spellbook.PermissionWriteContent) && !current.HasPermission(spellbook.PermissionWriteMedia)) {
		var p spellbook.Permission
		p = spellbook.PermissionWriteContent
		if !current.HasPermission(spellbook.PermissionWriteMedia) {
			p = spellbook.PermissionWriteMedia
		}
		return spellbook.NewPermissionError(spellbook.PermissionName(p))
	}

	other := Attachment{}
	if err := other.FromRepresentation(spellbook.RepresentationTypeJSON, bundle); err != nil {
		return spellbook.NewFieldError("", fmt.Errorf("bad json %s", string(bundle)))
	}

	attachment := res.(*Attachment)
	attachment.Name = other.Name
	attachment.Description = other.Description
	attachment.ResourceUrl = other.ResourceUrl
	attachment.ResourceThumbUrl = other.ResourceThumbUrl
	attachment.Group = other.Group
	attachment.ParentType = other.ParentType
	attachment.DisplayOrder = other.DisplayOrder

	// test the attachment parent type
	if sa := SupportedAttachmentsFromContext(ctx); sa != nil {
		if !sa.IsSupported(attachment) {
			msg := fmt.Sprintf("unsupported parent type %q for attachment", attachment.ParentType)
			return spellbook.NewFieldError("parentType", errors.New(msg))
		}
	}

	attachment.setParentKey(other.ParentKey)

	if attachment.ParentKey == "" {
		msg := fmt.Sprintf("attachment parent can't be empty. Use %s as a parent for global attachments", AttachmentGlobalParent)
		return spellbook.NewFieldError("parent", errors.New(msg))
	}

	if attachment.ResourceThumbUrl == "" {
		log.Infof(ctx, "No thumbnail provided for attachment %s, the image url will be used", attachment.Name)
		attachment.ResourceThumbUrl = attachment.ResourceUrl
	}

	attachment.Updated = time.Now().UTC()
	attachment.AltText = other.AltText

	return model.Update(ctx, attachment)
}

func (manager AttachmentManager) Delete(ctx context.Context, res spellbook.Resource) error {
	if current := spellbook.IdentityFromContext(ctx); current == nil || (!current.HasPermission(spellbook.PermissionWriteContent) && !current.HasPermission(spellbook.PermissionWriteMedia)) {
		var p spellbook.Permission
		p = spellbook.PermissionWriteContent
		if !current.HasPermission(spellbook.PermissionWriteMedia) {
			p = spellbook.PermissionWriteMedia
		}
		return spellbook.NewPermissionError(spellbook.PermissionName(p))
	}

	attachment := res.(*Attachment)
	err := model.Delete(ctx, attachment, nil)
	if err != nil {
		log.Errorf(ctx, "error deleting attachment %s: %s", attachment.Name, err.Error())
		return err
	}

	return nil
}
