package subscription

import (
	"context"
	"decodica.com/flamel/model"
	"decodica.com/spellbook"
	"errors"
	"fmt"
	"google.golang.org/appengine/log"
	"reflect"
	"sort"
	"strings"
	"time"
)

type SubscriptionController struct {
	*spellbook.RestController
}

func (co SubscriptionController) DefaultOffer() string {
	return "application/json"
}

func (co SubscriptionController) Offers() []string {
	return []string{"application/json", "text/csv"}
}

func NewSubscriptionController() SubscriptionController {
	return NewSubscriptionControllerWithKey("")
}

func NewSubscriptionControllerWithKey(key string) SubscriptionController {
	man := subscriptionManager{}
	handler := spellbook.BaseRestHandler{Manager: man}
	c := spellbook.NewRestController(handler)
	c.Key = key
	return SubscriptionController{c}
}

type subscriptionManager struct{}

func (manager subscriptionManager) NewResource(ctx context.Context) (spellbook.Resource, error) {
	return &Subscription{}, nil
}

func (manager subscriptionManager) FromId(ctx context.Context, strId string) (spellbook.Resource, error) {
	current := spellbook.IdentityFromContext(ctx)
	if !current.HasPermission(spellbook.PermissionReadSubscription) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadSubscription))
	}

	sub := Subscription{}
	if err := model.FromEncodedKey(ctx, &sub, strId); err != nil {
		log.Errorf(ctx, "could not retrieve subscription %s: %s", strId, err.Error())
		return nil, err
	}

	return &sub, nil
}

func (manager subscriptionManager) ListOf(ctx context.Context, opts spellbook.ListOptions) ([]spellbook.Resource, error) {
	current := spellbook.IdentityFromContext(ctx)
	if !current.HasPermission(spellbook.PermissionReadSubscription) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadSubscription))
	}

	var subscriptions []*Subscription
	q := model.NewQuery(&Subscription{})
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
	err := q.GetMulti(ctx, &subscriptions)
	if err != nil {
		return nil, err
	}

	resources := make([]spellbook.Resource, len(subscriptions))
	for i := range subscriptions {
		resources[i] = subscriptions[i]
	}

	return resources, nil
}

func (manager subscriptionManager) ListOfProperties(ctx context.Context, opts spellbook.ListOptions) ([]string, error) {
	current := spellbook.IdentityFromContext(ctx)
	if !current.HasPermission(spellbook.PermissionReadSubscription) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadSubscription))
	}

	a := []string{"Email"} // list property accepted
	name := opts.Property

	i := sort.Search(len(a), func(i int) bool { return name <= a[i] })
	if i < len(a) && a[i] == name {
		// found
	} else {
		return nil, errors.New("no property found")
	}

	var conts []*Subscription
	q := model.NewQuery(&Subscription{})
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

func (manager subscriptionManager) Create(ctx context.Context, res spellbook.Resource, bundle []byte) error {

	current := spellbook.IdentityFromContext(ctx)
	if current == nil || !current.HasPermission(spellbook.PermissionWriteMailMessage) {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionWriteSubscription))
	}

	subscription := res.(*Subscription)
	subscription.Created = time.Now().UTC()

	if subscription.Email == "" {
		msg := fmt.Sprintf("Email can't be empty")
		return spellbook.NewFieldError("Email", errors.New(msg))
	}
	if !strings.Contains(subscription.Email, "@") || !strings.Contains(subscription.Email, ".") {
		msg := fmt.Sprintf("Email not valid")
		return spellbook.NewFieldError("Email", errors.New(msg))
	}
	if subscription.Country == "" {
		msg := fmt.Sprintf("Country can't be empty")
		return spellbook.NewFieldError("Country", errors.New(msg))
	}
	if subscription.FirstName == "" {
		msg := fmt.Sprintf("FirstName can't be empty")
		return spellbook.NewFieldError("FirstName", errors.New(msg))
	}
	if subscription.LastName == "" {
		msg := fmt.Sprintf("LastName can't be empty")
		return spellbook.NewFieldError("LastName", errors.New(msg))
	}
	if subscription.Organization == "" {
		msg := fmt.Sprintf("Organization can't be empty")
		return spellbook.NewFieldError("Organization", errors.New(msg))
	}

	// list subscription
	var subscriptions []*Subscription
	q := model.NewQuery(&Subscription{})
	q = q.WithField("Email =", subscription.Email)
	err := q.GetMulti(ctx, &subscriptions)
	if err != nil {
		msg := fmt.Sprintf("Error retrieving list subscription %+v", err)
		return spellbook.NewFieldError("Suscription", errors.New(msg))
	}
	if len(subscriptions) > 0 {
		msg := fmt.Sprintf("Email already exist")
		return spellbook.NewFieldError("Email", errors.New(msg))
	}

	err = model.Create(ctx, subscription)
	if err != nil {
		log.Errorf(ctx, "error creating subscription %s: %s", subscription.Name, err)
		return err
	}

	return nil
}

func (manager subscriptionManager) Update(ctx context.Context, res spellbook.Resource, bundle []byte) error {
	current := spellbook.IdentityFromContext(ctx)
	if current == nil || !current.HasPermission(spellbook.PermissionWriteSubscription) {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionWriteSubscription))
	}

	other := Subscription{}
	if err := other.FromRepresentation(spellbook.RepresentationTypeJSON, bundle); err != nil {
		return spellbook.NewFieldError("", fmt.Errorf("invalid json %s: %s", string(bundle), err.Error()))
	}

	if other.Email == "" {
		msg := fmt.Sprintf("Email can't be empty")
		return spellbook.NewFieldError("Email", errors.New(msg))
	}
	if !strings.Contains(other.Email, "@") || !strings.Contains(other.Email, ".") {
		msg := fmt.Sprintf("Email not valid")
		return spellbook.NewFieldError("Email", errors.New(msg))
	}
	if other.Country == "" {
		msg := fmt.Sprintf("Country can't be empty")
		return spellbook.NewFieldError("Country", errors.New(msg))
	}
	if other.FirstName == "" {
		msg := fmt.Sprintf("FirstName can't be empty")
		return spellbook.NewFieldError("FirstName", errors.New(msg))
	}
	if other.LastName == "" {
		msg := fmt.Sprintf("LastName can't be empty")
		return spellbook.NewFieldError("LastName", errors.New(msg))
	}
	if other.Organization == "" {
		msg := fmt.Sprintf("Organization can't be empty")
		return spellbook.NewFieldError("Organization", errors.New(msg))
	}
	if other.Position == "" {
		msg := fmt.Sprintf("Position can't be empty")
		return spellbook.NewFieldError("Position", errors.New(msg))
	}

	subscription := res.(*Subscription)
	subscription.Email = other.Email
	subscription.Country = other.Country
	subscription.FirstName = other.FirstName
	subscription.LastName = other.LastName
	subscription.Organization = other.Organization
	subscription.Position = other.Position
	subscription.Notes = other.Notes
	subscription.Updated = time.Now().UTC()

	return model.Update(ctx, subscription)
}

func (manager subscriptionManager) Delete(ctx context.Context, res spellbook.Resource) error {
	if current := spellbook.IdentityFromContext(ctx); current == nil || !current.HasPermission(spellbook.PermissionWriteSubscription) {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionWriteSubscription))
	}

	subscription := res.(*Subscription)
	err := model.Delete(ctx, subscription, nil)
	if err != nil {
		log.Errorf(ctx, "error deleting subscription %s: %s", subscription.Name, err.Error())
		return err
	}

	return nil
}
