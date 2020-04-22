package content

import (
	"context"
	"decodica.com/spellbook"
	"decodica.com/spellbook/identity"
	"decodica.com/spellbook/sql"
	"errors"
	"fmt"
	"github.com/jinzhu/gorm"
	"google.golang.org/appengine/log"
	"strconv"
	"strings"
	"time"
)

func NewSqlContentController() *spellbook.RestController {
	return NewSqlContentControllerWithKey("")
}

func NewSqlContentControllerWithKey(key string) *spellbook.RestController {
	handler := spellbook.BaseRestHandler{Manager: SqlContentManager{}}
	c := spellbook.NewRestController(handler)
	c.Key = key
	return c
}

type SqlContentManager struct {
	ContentManager
}

func (manager SqlContentManager) NewResource(ctx context.Context) (spellbook.Resource, error) {
	return &Content{}, nil
}

func (manager SqlContentManager) FromId(ctx context.Context, id string) (spellbook.Resource, error) {

	if current := spellbook.IdentityFromContext(ctx); current == nil || !current.HasPermission(spellbook.PermissionReadContent) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadContent))
	}

	content := Content{}

	db := sql.FromContext(ctx)
	intId, err := strconv.Atoi(id)
	if err != nil {
		msg := "invalid id format: " + id + ". Id must be an int"
		log.Errorf(ctx, msg)
		return nil, spellbook.NewFieldError("id", errors.New(msg))
	}

	//db = db.First(&content, intId).Related(&content.Attachments, "parent_id")
	db = db.First(&content, intId).Where("parent_type = ?", AttachmentParentTypeContent).Order("display_order asc")
	if err := db.Related(&content.Attachments, "parent_id").Error; err != nil {
		log.Errorf(ctx, "error retrieving attachment %d: %s", intId, err)
		return nil, err
	}

	return &content, nil
}

func (manager SqlContentManager) ListOf(ctx context.Context, opts spellbook.ListOptions) ([]spellbook.Resource, error) {

	if current := spellbook.IdentityFromContext(ctx); current == nil || !current.HasPermission(spellbook.PermissionReadContent) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadContent))
	}

	var conts []*Content

	db := sql.FromContext(ctx)
	db = db.Offset(opts.Page * opts.Size)

	db = db.Where(sql.FiltersToCondition(opts.Filters))

	if opts.Order != "" {
		dir := " asc"
		if opts.Descending {
			dir = " desc"
		}
		db = db.Order(fmt.Sprintf("%q %s", strings.ToLower(opts.Order), dir))
	}

	db = db.Limit(opts.Size + 1)
	if res := db.Find(&conts); res.Error != nil {
		log.Errorf(ctx, "error retrieving content: %s", res.Error.Error())
		return nil, res.Error
	}

	resources := make([]spellbook.Resource, len(conts))
	for i := range conts {
		resources[i] = conts[i]
	}
	return resources, nil
}

func (manager SqlContentManager) ListOfProperties(ctx context.Context, opts spellbook.ListOptions) ([]string, error) {

	if current := spellbook.IdentityFromContext(ctx); current == nil || !current.HasPermission(spellbook.PermissionReadContent) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadContent))
	}

	var property string
	switch opts.Property {
	case "Category":
		property = "category"
	case "Locale":
		property = "locale"
	case "Name":
		property = "name"
	case "Topic":
		property = "topic"
	case "":
		return nil, spellbook.NewFieldError("property", fmt.Errorf("properties can't have no name"))
	default:
		return nil, gorm.ErrRecordNotFound
	}

	var result []string
	db := sql.FromContext(ctx)
	rows, err := db.Raw(fmt.Sprintf("SELECT DISTINCT %s FROM contents", property)).Rows()
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

func (manager SqlContentManager) Create(ctx context.Context, res spellbook.Resource, bundle []byte) error {
	current := spellbook.IdentityFromContext(ctx)
	if current == nil || !current.HasPermission(spellbook.PermissionWriteContent) {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionWriteContent))
	}

	content := res.(*Content)

	content.Created = time.Now().UTC()

	if content.IdTranslate == "" {
		content.IdTranslate = time.Now().Format(time.RFC3339Nano)
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

	if !content.StartDate.IsZero() && !content.EndDate.IsZero() && content.EndDate.Before(content.StartDate) {
		msg := fmt.Sprintf("end date %v can't be before start date %v", content.EndDate, content.StartDate)
		return spellbook.NewFieldError("endDate", errors.New(msg))
	}

	if user, ok := current.(identity.User); ok {
		content.Author = user.Username()
	}

	db := sql.FromContext(ctx)
	if res := db.Create(&content); res.Error != nil {
		log.Errorf(ctx, "error creating content %s: %s", content.Id(), res.Error)
		return res.Error
	}

	return nil
}

func (manager SqlContentManager) Update(ctx context.Context, res spellbook.Resource, bundle []byte) error {

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

	// check if content locale is

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

	db := sql.FromContext(ctx)

	if res := db.Save(content); res.Error != nil {
		return fmt.Errorf("error updating post %s: %s", content.Slug, res.Error)
	}

	return nil
}

func (manager SqlContentManager) Delete(ctx context.Context, res spellbook.Resource) error {

	if current := spellbook.IdentityFromContext(ctx); current == nil || !current.HasPermission(spellbook.PermissionWriteContent) {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionWriteContent))
	}

	content := res.(*Content)
	db := sql.FromContext(ctx)
	if res := db.Delete(content); res.Error != nil {
		log.Errorf(ctx, "error deleting content %s: %s", content.Slug, res.Error)
		return res.Error
	}

	return nil
}
