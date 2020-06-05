package identity

import (
	"cloud.google.com/go/datastore"
	"context"
	"decodica.com/spellbook"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/decodica/model/v2"
	"google.golang.org/appengine/log"
	"reflect"
	"sort"
)

type UserManager struct{}

func NewUserController() *spellbook.RestController {
	return NewUserControllerWithKey("")
}

func NewUserControllerWithKey(key string) *spellbook.RestController {
	manager := UserManager{}
	handler := spellbook.BaseRestHandler{Manager: manager}
	c := spellbook.NewRestController(handler)
	c.Key = key
	return c
}

func (manager UserManager) NewResource(ctx context.Context) (spellbook.Resource, error) {
	return &User{}, nil
}

func (manager UserManager) FromId(ctx context.Context, id string) (spellbook.Resource, error) {
	current := spellbook.IdentityFromContext(ctx)
	if current == nil {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadUser))
	}

	user, ok := current.(User)
	if ok && id == user.Id() {
		return &user, nil
	}

	if !current.HasPermission(spellbook.PermissionReadUser) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadUser))
	}

	us := User{}
	if err := model.FromStringID(ctx, &us, id, nil); err != nil {
		log.Errorf(ctx, "could not retrieve user %s: %s", id, err.Error())
		return nil, err
	}

	return &us, nil
}

func (manager UserManager) ListOf(ctx context.Context, opts spellbook.ListOptions) ([]spellbook.Resource, error) {

	if current := spellbook.IdentityFromContext(ctx); current == nil || !current.HasPermission(spellbook.PermissionReadUser) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadUser))
	}

	var users []*User
	q := model.NewQuery(&User{})
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
	err := q.GetMulti(ctx, &users)
	if err != nil {
		return nil, err
	}

	resources := make([]spellbook.Resource, len(users))
	for i := range users {
		resources[i] = users[i]
	}

	return resources, nil
}

func (manager UserManager) ListOfProperties(ctx context.Context, opts spellbook.ListOptions) ([]string, error) {
	if current := spellbook.IdentityFromContext(ctx); current == nil || !current.HasPermission(spellbook.PermissionReadUser) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadUser))
	}

	a := []string{"Group"}
	name := opts.Property

	i := sort.Search(len(a), func(i int) bool { return name <= a[i] })
	if i < len(a) && a[i] == name {
		// found
	} else {
		return nil, errors.New("no property found")
	}

	var conts []*User
	q := model.NewQuery(&User{})
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

func (manager UserManager) Create(ctx context.Context, res spellbook.Resource, bundle []byte) error {

	current := spellbook.IdentityFromContext(ctx)
	if current == nil || !current.HasPermission(spellbook.PermissionWriteUser) {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionWriteUser))
	}

	user := res.(*User)

	// todo: check if this is the case. Ideally in create this should never happen
	if user.Username() != "" {
		return spellbook.NewFieldError("username", fmt.Errorf("user already exists"))
	}

	meta := struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{}

	err := json.Unmarshal(bundle, &meta)
	if err != nil {
		return spellbook.NewFieldError("json", fmt.Errorf("invalid json: %s", string(bundle)))
	}

	username := meta.Username
	username = SanitizeUserName(username)

	uf := spellbook.NewRawField("username", true, username)
	uf.AddValidator(spellbook.DatastoreKeyNameValidator{})

	// validate the username. Accepted values for the username are implementation dependent
	if err := uf.Validate(); err != nil {
		msg := fmt.Sprintf("invalid username %s", user.Username())
		return spellbook.NewFieldError("username", errors.New(msg))
	}

	pf := spellbook.NewRawField("password", true, meta.Password)
	pf.AddValidator(spellbook.LenValidator{MinLen: 8})

	if err := pf.Validate(); err != nil {
		msg := fmt.Sprintf("invalid password %s for username %s", meta.Password, username)
		return spellbook.NewFieldError("password", errors.New(msg))
	}

	if !current.HasPermission(spellbook.PermissionEditPermissions) {
		// user without the EditPermission perm can only enable or disable a user
		if !((len(user.Permissions()) == 1 && user.IsEnabled()) || (len(user.Permissions()) == 0 && !user.IsEnabled())) {
			return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionEditPermissions))
		}
	}

	// check for user existence
	err = model.FromStringID(ctx, &User{}, username, nil)

	if err == nil {
		// user already exists
		msg := fmt.Sprintf("user %s already exists.", username)
		return spellbook.NewFieldError("user", errors.New(msg))
	}

	if err != datastore.ErrNoSuchEntity {
		// generic datastore error
		msg := fmt.Sprintf("error retrieving user with username %s: %s", username, err.Error())
		return spellbook.NewFieldError("user", errors.New(msg))
	}

	salt := spellbook.Application().Options().Salt
	user.Password = HashPassword(meta.Password, salt)

	opts := model.CreateOptions{}
	opts.WithStringId(username)

	err = model.CreateWithOptions(ctx, user, &opts)
	if err != nil {
		return fmt.Errorf("error creating user %s: %s", user.Name, err)
	}

	return nil
}

func (manager UserManager) Update(ctx context.Context, res spellbook.Resource, bundle []byte) error {
	current := spellbook.IdentityFromContext(ctx)
	if !current.HasPermission(spellbook.PermissionWriteUser) {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionWriteUser))
	}

	o, _ := manager.NewResource(ctx)
	if err := o.FromRepresentation(spellbook.RepresentationTypeJSON, bundle); err != nil {
		return spellbook.NewFieldError("", fmt.Errorf("invalid json %s: %s", string(bundle), err.Error()))
	}

	other := o.(*User)
	user := res.(*User)
	user.Name = other.Name

	tkn, _ := TokenManager{}.NewResource(ctx)
	token := tkn.(*Token)
	if err := token.FromRepresentation(spellbook.RepresentationTypeJSON, bundle); err == nil {
		if token.Password != "" {
			pf := spellbook.NewRawField("password", true, token.Password)
			pf.AddValidator(spellbook.LenValidator{MinLen: 8})

			if err := pf.Validate(); err != nil {
				msg := fmt.Sprintf("invalid password %s for username %s", token.Password, other.Username())
				return spellbook.NewFieldError("user", errors.New(msg))
			}
			salt := spellbook.Application().Options().Salt
			user.Password = HashPassword(token.Password, salt)
		}
	}

	if other.Email != "" {
		ef := spellbook.NewRawField("email", true, other.Email)
		if err := ef.Validate(); err != nil {
			msg := fmt.Sprintf("invalid email address: %s", other.Email)
			return spellbook.NewFieldError("user", errors.New(msg))
		}
		user.Email = other.Email
	}

	if !current.HasPermission(spellbook.PermissionEditPermissions) && other.ChangedPermission(*user) {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionEditPermissions))
	}

	user.Name = other.Name
	user.Surname = other.Surname
	user.Permission = other.Permission

	return model.Update(ctx, user)
}

func (manager UserManager) Delete(ctx context.Context, res spellbook.Resource) error {
	current := spellbook.IdentityFromContext(ctx)
	if !current.HasPermission(spellbook.PermissionWriteUser) {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionWriteUser))
	}

	user := res.(*User)
	err := model.Delete(ctx, user, nil)
	if err != nil {
		return fmt.Errorf("error deleting user %s: %s", user.Name, err.Error())
	}

	return nil
}
