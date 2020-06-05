package content

import (
	"context"
	"decodica.com/spellbook"
	"errors"
	"fmt"
	"github.com/decodica/model/v2"
	"google.golang.org/appengine/log"
	"reflect"
	"sort"
	"strconv"
	"time"
)

func NewPlaceController() *spellbook.RestController {
	return NewPlaceControllerWithKey("")
}

func NewPlaceControllerWithKey(key string) *spellbook.RestController {
	man := PlaceManager{}
	handler := spellbook.BaseRestHandler{Manager: man}
	c := spellbook.NewRestController(handler)
	c.Key = key
	return c
}

type PlaceManager struct{}

func (manager PlaceManager) NewResource(ctx context.Context) (spellbook.Resource, error) {
	return &Place{}, nil
}

func (manager PlaceManager) FromId(ctx context.Context, strId string) (spellbook.Resource, error) {

	if current := spellbook.IdentityFromContext(ctx); current == nil || !current.HasPermission(spellbook.PermissionReadPlace) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadPlace))
	}

	id, err := strconv.ParseInt(strId, 10, 64)
	if err != nil {
		return nil, spellbook.NewFieldError(strId, err)
	}

	att := Place{}
	if err := model.FromIntID(ctx, &att, id, nil); err != nil {
		log.Errorf(ctx, "could not retrieve place %s: %s", id, err.Error())
		return nil, err
	}

	return &att, nil
}

func (manager PlaceManager) ListOf(ctx context.Context, opts spellbook.ListOptions) ([]spellbook.Resource, error) {
	if current := spellbook.IdentityFromContext(ctx); current == nil || !current.HasPermission(spellbook.PermissionReadPlace) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadPlace))
	}

	var places []*Place
	q := model.NewQuery(&Place{})
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
	err := q.GetMulti(ctx, &places)
	if err != nil {
		return nil, err
	}

	resources := make([]spellbook.Resource, len(places))
	for i := range places {
		resources[i] = places[i]
	}

	return resources, nil
}

func (manager PlaceManager) ListOfProperties(ctx context.Context, opts spellbook.ListOptions) ([]string, error) {
	if current := spellbook.IdentityFromContext(ctx); current == nil || !current.HasPermission(spellbook.PermissionReadPlace) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadPlace))
	}

	a := []string{"City", "Name"} // list property accepted
	name := opts.Property

	i := sort.Search(len(a), func(i int) bool { return name <= a[i] })
	if i < len(a) && a[i] == name {
		// found
	} else {
		return nil, errors.New("no property found")
	}

	var conts []*Place
	q := model.NewQuery(&Place{})
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

func (manager PlaceManager) Create(ctx context.Context, res spellbook.Resource, bundle []byte) error {
	current := spellbook.IdentityFromContext(ctx)
	if current == nil || !current.HasPermission(spellbook.PermissionWritePlace) {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionWritePlace))
	}

	place := res.(*Place)

	if place.Address == "" || !place.Position.Valid() {
		return spellbook.NewFieldError("address", errors.New("address and position can't be empty"))
	}

	place.Created = time.Now().UTC()

	err := model.Create(ctx, place)
	if err != nil {
		log.Errorf(ctx, "error creating place %s: %s", place.Name, err)
		return err
	}

	return nil
}

func (manager PlaceManager) Update(ctx context.Context, res spellbook.Resource, bundle []byte) error {
	current := spellbook.IdentityFromContext(ctx)
	if current == nil || !current.HasPermission(spellbook.PermissionWritePlace) {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionWritePlace))
	}

	other := Place{}
	if err := other.FromRepresentation(spellbook.RepresentationTypeJSON, bundle); err != nil {
		return spellbook.NewFieldError("", fmt.Errorf("bad json %s", string(bundle)))
	}

	place := res.(*Place)
	place.Name = other.Name
	place.Address = other.Address
	place.City = other.City
	place.Street = other.Street
	place.StreetNumber = other.StreetNumber
	place.Area = other.Area
	place.PostalCode = other.PostalCode
	place.Country = other.Country
	place.Description = other.Description
	place.Position = other.Position
	place.Phone = other.Phone
	place.Website = other.Website
	place.Updated = time.Now().UTC()

	if place.Address == "" || !place.Position.Valid() {
		return spellbook.NewFieldError("address", errors.New("address and position can't be empty"))
	}

	if err := model.Update(ctx, place); err != nil {
		return fmt.Errorf("error updating place %s: %s", place.Address, err)
	}

	return nil
}

func (manager PlaceManager) Delete(ctx context.Context, res spellbook.Resource) error {
	current := spellbook.IdentityFromContext(ctx)
	if current == nil || !current.HasPermission(spellbook.PermissionWritePlace) {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionWritePlace))
	}

	place := res.(*Place)
	err := model.Delete(ctx, place, nil)
	if err != nil {
		log.Errorf(ctx, "error deleting place %s: %s", place.Name, err.Error())
		return err
	}

	return nil
}
