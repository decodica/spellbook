package identity

import (
	"context"
	"decodica.com/spellbook"
	"decodica.com/spellbook/sql"
	"fmt"
	"github.com/decodica/model/v2"
	"github.com/jinzhu/gorm"
)

func NewSqlTokenController() *spellbook.RestController {
	handler := spellbook.BaseRestHandler{}
	handler.Manager = NewDefaultSqlTokenManager()
	return spellbook.NewRestController(handler)
}

func NewSqlTokenControllerWithKey(key string) *spellbook.RestController {
	handler := spellbook.BaseRestHandler{Manager: NewDefaultSqlTokenManager()}
	c := spellbook.NewRestController(handler)
	c.Key = key
	return c
}

type SqlTokenManager struct {
	UserManager spellbook.Manager
}

func NewDefaultSqlTokenManager() SqlTokenManager {
	return SqlTokenManager{DefaultSqlUserManager}
}

func (manager SqlTokenManager) NewResource(ctx context.Context) (spellbook.Resource, error) {
	return &Token{}, nil
}

func (manager SqlTokenManager) FromId(ctx context.Context, id string) (spellbook.Resource, error) {
	current := spellbook.IdentityFromContext(ctx)
	if current == nil {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadUser))
	}

	user, ok := current.(User)
	if ok && id == user.Id() {
		return &user, nil
	}

	return manager.UserManager.FromId(ctx, id)
}

func (manager SqlTokenManager) ListOf(ctx context.Context, opts spellbook.ListOptions) ([]spellbook.Resource, error) {
	return nil, spellbook.NewUnsupportedError()
}

func (manager SqlTokenManager) ListOfProperties(ctx context.Context, opts spellbook.ListOptions) ([]string, error) {
	return nil, spellbook.NewUnsupportedError()
}

func (manager SqlTokenManager) Create(ctx context.Context, res spellbook.Resource, bundle []byte) error {

	token := res.(*Token)

	// checks the provided credentials. If correct creates a token, saves the user and returns the token
	nick := spellbook.NewRawField("username", true, token.Username)
	if _, err := nick.Value(); err != nil {
		return spellbook.NewFieldError("username", err)
	}

	password := spellbook.NewRawField("password", true, token.Password)
	password.AddValidator(spellbook.LenValidator{MinLen: 8})
	if _, err := password.Value(); err != nil {
		return spellbook.NewFieldError("password", err)
	}

	u := &User{}
	db := sql.FromContext(ctx)
	err := db.Where("username = ?", token.Username).First(u).Error

	if err != nil {
		return err
	}

	salt := spellbook.Application().Options().Salt
	hp := HashPassword(token.Password, salt)
	if u.Password != hp {
		return gorm.ErrRecordNotFound
	}

	tv, err := u.GenerateToken()
	if err != nil {
		return fmt.Errorf("error generating token for user %s: %s", u.Username(), err.Error())
	}

	u.setToken(tv)
	err = db.Save(&u).Error
	if err != nil {
		return fmt.Errorf("error updating user token: %s", err.Error())
	}

	token.Value = u.Token

	return nil
}

func (manager SqlTokenManager) Update(ctx context.Context, res spellbook.Resource, bundle []byte) error {
	return spellbook.NewUnsupportedError()
}

func (manager SqlTokenManager) Delete(ctx context.Context, res spellbook.Resource) error {

	u := spellbook.IdentityFromContext(ctx)
	user, ok := u.(User)
	if !ok {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionEnabled))
	}

	user.setToken("")
	err := model.Update(ctx, &user)
	if err != nil {
		return err
	}

	return nil
}
