package content

import (
	"cloud.google.com/go/datastore"
	"context"
	"decodica.com/flamel/model"
	"decodica.com/spellbook"
	"decodica.com/spellbook/identity"
	"errors"
	"fmt"
	"google.golang.org/appengine/log"
	"reflect"
	"sort"
	"time"
)

func NewContentController() *spellbook.RestController {
	return NewContentControllerWithKey("")
}

func NewContentControllerWithKey(key string) *spellbook.RestController {
	handler := spellbook.BaseRestHandler{Manager: ContentManager{}}
	c := spellbook.NewRestController(handler)
	c.Key = key
	return c
}

type ContentManager struct{}

func (manager ContentManager) NewResource(ctx context.Context) (spellbook.Resource, error) {
	return &Content{}, nil
}

func (manager ContentManager) FromId(ctx context.Context, id string) (spellbook.Resource, error) {

	if current := spellbook.IdentityFromContext(ctx); current == nil || !current.HasPermission(spellbook.PermissionReadContent) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadContent))
	}

	cont := Content{}

	if err := model.FromEncodedKey(ctx, &cont, id); err != nil {
		log.Errorf(ctx, "could not retrieve content %s: %s", id, err.Error())
		return nil, err
	}

	// attachment
	q := model.NewQuery((*Attachment)(nil))
	q = q.WithField("ParentKey =", cont.Id())
	q = q.OrderBy("DisplayOrder", model.ASC)
	if err := q.GetMulti(ctx, &cont.Attachments); err != nil {
		log.Errorf(ctx, "could not retrieve content %s attachments: %s", id, err.Error())
		return nil, err
	}

	return &cont, nil
}

func (manager ContentManager) ListOf(ctx context.Context, opts spellbook.ListOptions) ([]spellbook.Resource, error) {

	if current := spellbook.IdentityFromContext(ctx); current == nil || !current.HasPermission(spellbook.PermissionReadContent) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadContent))
	}

	var conts []*Content
	q := model.NewQuery(&Content{})
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
	err := q.GetMulti(ctx, &conts)
	if err != nil {
		return nil, err
	}

	resources := make([]spellbook.Resource, len(conts))
	for i := range conts {
		resources[i] = conts[i]
	}

	return resources, nil
}

func (manager ContentManager) ListOfProperties(ctx context.Context, opts spellbook.ListOptions) ([]string, error) {

	if current := spellbook.IdentityFromContext(ctx); current == nil || !current.HasPermission(spellbook.PermissionReadContent) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadContent))
	}

	a := []string{"Category", "Locale", "Name", "Topic"} // list property accepted
	name := opts.Property

	if name == "" {
		return nil, spellbook.NewFieldError("property", fmt.Errorf("empty property"))
	}

	i := sort.Search(len(a), func(i int) bool { return name <= a[i] })
	if i == len(a) {
		return nil, datastore.ErrNoSuchEntity
	}

	var conts []*Content
	q := model.NewQuery(&Content{})
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

func (manager ContentManager) Create(ctx context.Context, res spellbook.Resource, bundle []byte) error {

	current := spellbook.IdentityFromContext(ctx)
	if current == nil || !current.HasPermission(spellbook.PermissionWriteContent) {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionWriteContent))
	}

	content := res.(*Content)

	content.Created = time.Now().UTC()
	if content.IdTranslate == "" {
		content.IdTranslate = time.Now().Format(time.RFC3339Nano)
	} else {
		// check same idTranslate and Locale
		q := model.NewQuery((*Content)(nil))
		q = q.WithField("IdTranslate =", content.IdTranslate)
		q = q.WithField("Locale = ", content.Locale)
		count, err := q.Count(ctx)
		if err != nil {
			return spellbook.NewFieldError("locale", fmt.Errorf("error verifying locale translate: %s", err.Error()))
		}
		if count > 0 {
			msg := fmt.Sprintf("a content with IdTranslate '%s' already exists. Locale must be unique.", content.Locale)
			return spellbook.NewFieldError("locale", errors.New(msg))
		}
	}
	content.Revision = 1

	if content.IsPublished() {
		content.PublicationState = PublicationStatePublished
		content.Published = time.Now().UTC()
	} else {
		content.PublicationState = PublicationStateUnpublished
	}

	if content.Type == "" {
		return spellbook.NewFieldError("type", errors.New("type can't be empty"))
	}

	if content.Title == "" {
		return spellbook.NewFieldError("title", errors.New("title can't be empty"))
	}

	if content.Slug == "" && content.Code == "" {
		return spellbook.NewFieldError("slug", fmt.Errorf("non special content can't have an empty slug"))
	}

	// if the same slug already exists, we must return
	// otherwise we would overwrite an existing entry, which is not in the spirit of the create method
	q := model.NewQuery((*Content)(nil))

	// if is a special content, we check that the content doesn't already exist
	reason := "code"
	if content.Code == "" {
		q = q.WithField("Slug =", content.Slug)
		q = q.WithField("Locale = ", content.Locale)
		reason = "slug"
	} else {
		q = q.WithField("Code =", content.Code)
		q = q.WithField("Locale = ", content.Locale)
	}
	count, err := q.Count(ctx)
	if err != nil {
		return spellbook.NewFieldError("slug", fmt.Errorf("error verifying slug uniqueness: %s", err.Error()))
	}

	if count > 0 {
		msg := fmt.Sprintf("a content with the same %s already exists.", reason)
		return spellbook.NewFieldError("slug", errors.New(msg))
	}

	if !content.StartDate.IsZero() && !content.EndDate.IsZero() && content.EndDate.Before(content.StartDate) {
		msg := fmt.Sprintf("end date %v can't be before start date %v", content.EndDate, content.StartDate)
		return spellbook.NewFieldError("endDate", errors.New(msg))
	}

	if user, ok := current.(identity.User); ok {
		content.Author = user.Username()
	}

	// // WARNING: the volatile field Multimedia because Memcache (Gob)
	//	can't ignore field
	tmp := content.Attachments
	content.Attachments = nil

	err = model.Create(ctx, content)
	if err != nil {
		log.Errorf(ctx, "error creating post %s: %s", content.Slug, err)
		return err
	}

	// return the swapped multimedia value
	content.Attachments = tmp

	return nil
}

func (manager ContentManager) Update(ctx context.Context, res spellbook.Resource, bundle []byte) error {

	current := spellbook.IdentityFromContext(ctx)

	if current == nil || !current.HasPermission(spellbook.PermissionWriteContent) {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionWriteContent))
	}

	content := res.(*Content)

	other := &Content{}
	if err := other.FromRepresentation(spellbook.RepresentationTypeJSON, bundle); err != nil {
		return spellbook.NewFieldError("", fmt.Errorf("invalid json for content %s: %s", content.StringID(), err.Error()))
	}

	if other.Type == "" {
		return spellbook.NewFieldError("type", errors.New("type can't be empty"))
	}

	if other.Title == "" {
		return spellbook.NewFieldError("title", errors.New("title can't be empty"))
	}

	// if the same slug already exists, we must return
	// otherwise we would overwrite an existing entry, which is not in the spirit of the create method
	q := model.NewQuery((*Content)(nil))

	if other.Slug == "" && other.Code == "" {
		return spellbook.NewFieldError("slug", fmt.Errorf("non special content can't have an empty slug"))
	}

	reason := "code"
	if other.Code == "" {
		q = q.WithField("Slug =", other.Slug)
		q = q.WithField("Locale = ", other.Locale)
		reason = "slug"
	} else {
		q = q.WithField("Code =", other.Code)
		q = q.WithField("Locale = ", other.Locale)
	}

	compare := Content{}
	err := q.First(ctx, &compare)
	if err == nil && compare.EncodedKey() != content.EncodedKey() {
		return spellbook.NewFieldError("slug", fmt.Errorf("a content with the same %s already exists", reason))
	}

	if err != nil && err != datastore.ErrNoSuchEntity {
		return spellbook.NewFieldError("slug", fmt.Errorf("error verifying content correctness: %s", err.Error()))
	}

	content.Type = other.Type
	content.Title = other.Title
	content.Subtitle = other.Subtitle
	content.Category = other.Category
	content.Topic = other.Topic
	content.Locale = other.Locale
	content.Description = other.Description
	content.setCode(other.Code)
	content.Body = other.Body
	content.Cover = other.Cover
	content.Revision = other.Revision
	content.Editor = other.Editor
	content.Order = other.Order
	content.Updated = time.Now().UTC()
	content.Tags = other.Tags
	content.setSlug(other.Slug)
	content.ParentKey = other.ParentKey

	if !other.IsPublished() {
		// not set
		content.Published = time.Time{} // zero
	} else {
		// set
		// check previous data
		if !content.IsPublished() {
			content.Published = time.Now().UTC()
		}
	}

	if content.IsPublished() {
		content.PublicationState = PublicationStatePublished
	} else {
		content.PublicationState = PublicationStateUnpublished
	}

	if !other.StartDate.IsZero() && !other.EndDate.IsZero() && other.EndDate.Before(other.StartDate) {
		msg := fmt.Sprintf("end date %v can't be before start date %v", other.EndDate, other.StartDate)
		return spellbook.NewFieldError("endDate", errors.New(msg))
	}

	content.StartDate = other.StartDate
	content.EndDate = other.EndDate

	if user, ok := current.(identity.User); ok {
		content.Author = user.Username()
	}

	tmp := content.Attachments
	content.Attachments = nil

	if err := model.Update(ctx, content); err != nil {
		return fmt.Errorf("error updating post %s: %s", content.Slug, err)
	}

	// return the swapped multimedia value
	content.Attachments = tmp

	return nil
}

func (manager ContentManager) Delete(ctx context.Context, res spellbook.Resource) error {

	if current := spellbook.IdentityFromContext(ctx); current == nil || !current.HasPermission(spellbook.PermissionWriteContent) {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionWriteContent))
	}

	content := res.(*Content)
	err := model.Delete(ctx, content, nil)
	if err != nil {
		log.Errorf(ctx, "error deleting content %s: %s", content.Slug, err.Error())
		return err
	}

	// delete attachments with parent = slug
	attachments := make([]*Attachment, 0, 0)
	q := model.NewQuery(&Attachment{})
	q.WithField("ParentKey =", content.EncodedKey())
	err = q.GetMulti(ctx, &attachments)
	if err != nil {
		log.Errorf(ctx, "error retrieving attachments: %s", err)
		return err
	}

	for _, att := range attachments {
		err = model.Delete(ctx, att, nil)
		if err != nil {
			log.Errorf(ctx, "error deleting attachment %s: %s", att.Name, err.Error())
			return err
		}
	}

	return nil
}
