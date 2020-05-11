package content

import (
	"context"
	"decodica.com/spellbook"
	"decodica.com/spellbook/identity"
	"decodica.com/spellbook/sql"
	"errors"
	"fmt"
	"google.golang.org/appengine/log"
	"strconv"
	"strings"
	"time"
)

func NewSqlAttachmentController() *spellbook.RestController {
	return NewSqlAttachmentControllerWithKey("")
}

func NewSqlAttachmentControllerWithKey(key string) *spellbook.RestController {
	handler := spellbook.BaseRestHandler{Manager: SqlAttachmentManager{}}
	c := spellbook.NewRestController(handler)
	c.Key = key
	return c
}

type SqlAttachmentManager struct{}

func (manager SqlAttachmentManager) NewResource(ctx context.Context) (spellbook.Resource, error) {
	return &Attachment{}, nil
}

func (manager SqlAttachmentManager) FromId(ctx context.Context, strId string) (spellbook.Resource, error) {

	if current := spellbook.IdentityFromContext(ctx); current == nil || (!current.HasPermission(spellbook.PermissionReadContent) && !current.HasPermission(spellbook.PermissionReadMedia)) {
		var p spellbook.Permission
		p = spellbook.PermissionReadContent
		if !current.HasPermission(spellbook.PermissionReadMedia) {
			p = spellbook.PermissionReadMedia
		}
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(p))
	}

	id, err := strconv.ParseInt(strId, 10, 64)
	if err != nil {
		return nil, spellbook.NewFieldError(strId, err)
	}

	att := Attachment{}

	db := sql.FromContext(ctx)
	if res := db.First(&att, id); res.Error != nil {
		log.Errorf(ctx, "could not retrieve attachment %s: %s", id, res.Error.Error())
		return nil, err
	}

	return &att, nil
}

func (manager SqlAttachmentManager) ListOf(ctx context.Context, opts spellbook.ListOptions) ([]spellbook.Resource, error) {
	if current := spellbook.IdentityFromContext(ctx); current == nil || (!current.HasPermission(spellbook.PermissionReadContent) && !current.HasPermission(spellbook.PermissionReadMedia)) {
		var p spellbook.Permission
		p = spellbook.PermissionReadContent
		if !current.HasPermission(spellbook.PermissionReadMedia) {
			p = spellbook.PermissionReadMedia
		}
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(p))
	}

	var attachments []*Attachment
	db := sql.FromContext(ctx)
	db = db.Offset(opts.Page * opts.Size)

	db = db.Where(sql.FiltersToCondition(opts.Filters, nil))

	if opts.Order != "" {
		dir := " asc"
		if opts.Descending {
			dir = " desc"
		}
		db = db.Order(fmt.Sprintf("%q %s", strings.ToLower(opts.Order), dir))
	}

	db = db.Limit(opts.Size + 1)
	if res := db.Find(&attachments); res.Error != nil {
		log.Errorf(ctx, "error retrieving content: %s", res.Error.Error())
		return nil, res.Error
	}

	resources := make([]spellbook.Resource, len(attachments))
	for i := range attachments {
		resources[i] = attachments[i]
	}
	return resources, nil
}

func (manager SqlAttachmentManager) ListOfProperties(ctx context.Context, opts spellbook.ListOptions) ([]string, error) {

	if current := spellbook.IdentityFromContext(ctx); current == nil || !current.HasPermission(spellbook.PermissionReadMedia) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadMedia))
	}

	var property string
	switch opts.Property {
	case "Group":
		property = "group"
	case "ParentKey":
		property = "parent_key"
	default:
		return nil, errors.New("no property found")
	}

	var result []string
	db := sql.FromContext(ctx)
	rows, err := db.Raw(fmt.Sprintf("SELECT DISTINCT %q FROM attachments", property)).Rows()
	if err != nil {
		log.Errorf(ctx, "error retrieving property list for property %s: %s", property, err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			log.Errorf(ctx, "error retrieving property %s occurrencies: %s", property, err)
			return nil, err
		}
		result = append(result, p)
	}
	return result, nil
}

func (manager SqlAttachmentManager) Create(ctx context.Context, res spellbook.Resource, bundle []byte) error {
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

	db := sql.FromContext(ctx)
	if res := db.Create(&attachment); res.Error != nil {
		log.Errorf(ctx, "error creating post %s: %s", attachment.Name, res.Error)
		return res.Error
	}

	return nil
}

func (manager SqlAttachmentManager) Update(ctx context.Context, res spellbook.Resource, bundle []byte) error {
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

	if attachment.ResourceThumbUrl == "" {
		log.Infof(ctx, "No thumbnail provided for attachment %s, the image url will be used", attachment.Name)
		attachment.ResourceThumbUrl = attachment.ResourceUrl
	}

	attachment.setParentKey(other.ParentKey)

	if attachment.ParentKey == "" {
		msg := fmt.Sprintf("attachment parent can't be empty. Use %s as a parent for global attachments", AttachmentGlobalParent)
		return spellbook.NewFieldError("parent", errors.New(msg))
	}

	attachment.Updated = time.Now().UTC()
	attachment.AltText = other.AltText

	db := sql.FromContext(ctx)
	if res := db.Save(&attachment); res.Error != nil {
		log.Errorf(ctx, "error updating attachment %s: %s", attachment.Name, res.Error.Error())
		return res.Error
	}
	return nil
}

func (manager SqlAttachmentManager) Delete(ctx context.Context, res spellbook.Resource) error {
	if current := spellbook.IdentityFromContext(ctx); current == nil || (!current.HasPermission(spellbook.PermissionWriteContent) && !current.HasPermission(spellbook.PermissionWriteMedia)) {
		var p spellbook.Permission
		p = spellbook.PermissionWriteContent
		if !current.HasPermission(spellbook.PermissionWriteMedia) {
			p = spellbook.PermissionWriteMedia
		}
		return spellbook.NewPermissionError(spellbook.PermissionName(p))
	}

	attachment := res.(*Attachment)
	db := sql.FromContext(ctx)
	if res := db.Delete(attachment); res.Error != nil {
		log.Errorf(ctx, "error deleting attachment %s: %s", attachment.Name, res.Error.Error())
		return res.Error
	}

	return nil
}
