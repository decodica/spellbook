package identity

import (
	"context"
	"decodica.com/spellbook"
	"decodica.com/spellbook/sql"
	"encoding/json"
	"errors"
	"fmt"
	"google.golang.org/appengine/log"
	"strings"
)

type SqlUserManager struct{}

var DefaultSqlUserManager = SqlUserManager{}

func NewSqlUserController() *spellbook.RestController {
	return NewSqlUserControllerWithKey("")
}

func NewSqlUserControllerWithKey(key string) *spellbook.RestController {
	manager := SqlUserManager{}
	handler := spellbook.BaseRestHandler{Manager: manager}
	c := spellbook.NewRestController(handler)
	c.Key = key
	return c
}

func (manager SqlUserManager) NewResource(ctx context.Context) (spellbook.Resource, error) {
	return &User{}, nil
}

func (manager SqlUserManager) FromId(ctx context.Context, id string) (spellbook.Resource, error) {
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
	db := sql.FromContext(ctx)
	if err := db.Where("username =  ?", id).First(&us).Error; err != nil {
		log.Errorf(ctx, "could not retrieve user %s: %s", id, err.Error())
		return nil, err
	}

	return &us, nil
}

func (manager SqlUserManager) ListOf(ctx context.Context, opts spellbook.ListOptions) ([]spellbook.Resource, error) {

	if current := spellbook.IdentityFromContext(ctx); current == nil || !current.HasPermission(spellbook.PermissionReadUser) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadUser))
	}

	var users []*User
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
	if res := db.Find(&users); res.Error != nil {
		log.Errorf(ctx, "error retrieving content: %s", res.Error.Error())
		return nil, res.Error
	}

	resources := make([]spellbook.Resource, len(users))
	for i := range users {
		resources[i] = users[i]
	}
	return resources, nil
}

func (manager SqlUserManager) ListOfProperties(ctx context.Context, opts spellbook.ListOptions) ([]string, error) {
	if current := spellbook.IdentityFromContext(ctx); current == nil || !current.HasPermission(spellbook.PermissionReadUser) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadUser))
	}

	return nil, spellbook.NewUnsupportedError()
}

func (manager SqlUserManager) Create(ctx context.Context, res spellbook.Resource, bundle []byte) error {

	current := spellbook.IdentityFromContext(ctx)
	if current == nil || !current.HasPermission(spellbook.PermissionWriteUser) {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionWriteUser))
	}

	user := res.(*User)

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

	salt := spellbook.Application().Options().Salt
	user.Password = HashPassword(meta.Password, salt)
	user.SqlUsername = username

	db := sql.FromContext(ctx)

	if err := db.Create(user).Error; err != nil {
		return fmt.Errorf("error creating user %s: %s", user.Name, err)
	}

	return nil
}

func (manager SqlUserManager) Update(ctx context.Context, res spellbook.Resource, bundle []byte) error {
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

	db := sql.FromContext(ctx)

	return db.Save(user).Error
}

func (manager SqlUserManager) Delete(ctx context.Context, res spellbook.Resource) error {
	current := spellbook.IdentityFromContext(ctx)
	if !current.HasPermission(spellbook.PermissionWriteUser) {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionWriteUser))
	}

	user := res.(*User)

	db := sql.FromContext(ctx)
	if err := db.Delete(&user).Error; err != nil {
		return fmt.Errorf("error deleting user %s: %s", user.Name, err.Error())
	}

	return nil
}
