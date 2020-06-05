package navigation

import (
	"cloud.google.com/go/datastore"
	"context"
	"decodica.com/spellbook"
	"errors"
	"fmt"
	"github.com/decodica/model/v2"
	"google.golang.org/appengine/log"
)

func NewPageController() *spellbook.RestController {
	return NewPageControllerWithKey("")
}

func NewPageControllerWithKey(key string) *spellbook.RestController {
	handler := spellbook.BaseRestHandler{Manager: PageManager{}}
	c := spellbook.NewRestController(handler)
	c.Key = key
	return c
}

type PageManager struct{}

func (manager PageManager) NewResource(ctx context.Context) (spellbook.Resource, error) {
	return &Page{}, nil
}

func (manager PageManager) FromId(ctx context.Context, id string) (spellbook.Resource, error) {
	if current := spellbook.IdentityFromContext(ctx); current == nil || !current.HasPermission(spellbook.PermissionReadPage) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadPage))
	}

	cont := Page{}
	if err := model.FromStringID(ctx, &cont, id, nil); err != nil {
		log.Errorf(ctx, "could not retrieve seo %s: %s", id, err.Error())
		return nil, err
	}

	return &cont, nil
}

func (manager PageManager) ListOf(ctx context.Context, opts spellbook.ListOptions) ([]spellbook.Resource, error) {
	if current := spellbook.IdentityFromContext(ctx); current == nil || !current.HasPermission(spellbook.PermissionReadPage) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadPage))
	}

	var conts []*Page
	q := model.NewQuery(&Page{})
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

func (manager PageManager) ListOfProperties(ctx context.Context, opts spellbook.ListOptions) ([]string, error) {
	return nil, spellbook.NewUnsupportedError()
}

func (manager PageManager) Create(ctx context.Context, res spellbook.Resource, bundle []byte) error {

	current := spellbook.IdentityFromContext(ctx)
	if current == nil || !current.HasPermission(spellbook.PermissionWritePage) {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionWritePage))
	}

	p := res.(*Page)

	// if the same seo already exists, we must return false
	existing, _ := manager.NewResource(ctx)
	err := model.FromStringID(ctx, existing.(*Page), PageId(p.Locale, p.Url), nil)
	if err == datastore.ErrNoSuchEntity {
		// we can create the new seo element if another one with the same code doesn't exists
		q := model.NewQuery((*Page)(nil))
		q.WithField("Code =", string(p.Code))
		q.WithField("Locale =", p.Locale)

		err = q.First(ctx, &Page{})

		// seo already exists for given code, can't create
		if err == nil {
			msg := fmt.Sprintf("a page for %q already exists.", p.Code)
			return spellbook.NewFieldError("", errors.New(msg))
		}

		if err == datastore.ErrNoSuchEntity {
			p.IsRoot = p.Url == rootUrl
			opts := model.NewCreateOptions()
			opts.WithStringId(PageId(p.Locale, p.Url))
			err = model.CreateWithOptions(ctx, p, &opts)
		}

		if err != nil {
			return err
		}

		InvalidateMenu(ctx)
		return nil
	}

	if err != nil {
		return err
	}

	// the page with the given url has already been allocated. can't create seo
	msg := fmt.Sprintf("a seo for url %q already exists.", p.Url)
	return spellbook.NewFieldError("", errors.New(msg))
}

func (manager PageManager) Update(ctx context.Context, res spellbook.Resource, bundle []byte) error {

	current := spellbook.IdentityFromContext(ctx)
	if current == nil || !current.HasPermission(spellbook.PermissionWritePage) {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionWritePage))
	}

	p := res.(*Page)

	or, _ := manager.NewResource(ctx)
	other := or.(*Page)

	if err := other.FromRepresentation(spellbook.RepresentationTypeJSON, bundle); err != nil {
		return spellbook.NewFieldError("", fmt.Errorf("invalid json for seo %q: %s", p.StringID(), err.Error()))
	}

	if err := model.FromStringID(ctx, p, PageId(other.Locale, other.Url), nil); err != nil {
		return spellbook.NewUnsupportedError()
	}

	q := model.NewQuery((*Page)(nil))
	q.WithField("Code =", other.Code)
	q.WithField("Locale =", other.Locale)

	existing := Page{}
	err := q.First(ctx, &existing)

	// seo already exists for given code, can't create
	if err == nil && existing.StringID() != PageId(other.Locale, other.Url) {
		msg := fmt.Sprintf("a page for %q already exists.", other.Code)
		return spellbook.NewFieldError("", errors.New(msg))
	}

	p.Order = other.Order
	p.Label = other.Label
	p.Title = other.Title
	p.MetaDesc = other.MetaDesc
	p.Code = other.Code

	if err := model.Update(ctx, p); err != nil {
		return fmt.Errorf("error updating seo with url %q: %s", p.Url, err)
	}

	InvalidateMenu(ctx)
	return nil
}

func (manager PageManager) Delete(ctx context.Context, res spellbook.Resource) error {

	p := res.(*Page)
	err := model.Delete(ctx, p, nil)
	if err != nil {
		log.Errorf(ctx, "error deleting seo with url %q: %s", p.Url, err.Error())
		return err
	}

	InvalidateMenu(ctx)
	return nil
}
