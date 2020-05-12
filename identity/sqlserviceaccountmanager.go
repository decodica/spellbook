package identity

import (
	"context"
	"decodica.com/spellbook"
	"decodica.com/spellbook/sql"
	"errors"
	"fmt"
	"google.golang.org/appengine/log"
	"strings"
	"time"
)


type SqlServiceAccountManager struct{
	tg TokenGenerator
}

func NewDefaultSqlServiceAccountManager() SqlServiceAccountManager {
	return SqlServiceAccountManager{ServiceAccountTokenGenerator{}}
}

func NewSqlServiceAccountController() *spellbook.RestController {
	return NewSqlServiceAccountControllerWithKey("")
}

func NewSqlServiceAccountControllerWithKey(key string) *spellbook.RestController {
	manager := NewDefaultSqlServiceAccountManager()
	handler := spellbook.BaseRestHandler{Manager: manager}
	c := spellbook.NewRestController(handler)
	c.Key = key
	return c
}

func (manager SqlServiceAccountManager) NewResource(ctx context.Context) (spellbook.Resource, error) {
	return &ServiceAccount{}, nil
}

func (manager SqlServiceAccountManager) FromId(ctx context.Context, id string) (spellbook.Resource, error) {
	current := spellbook.IdentityFromContext(ctx)
	if current == nil {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadUser))
	}

	sa, ok := current.(ServiceAccount)
	if ok && id == sa.Id() {
		return &sa, nil
	}

	if !current.HasPermission(spellbook.PermissionReadUser) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadUser))
	}


	db := sql.FromContext(ctx)
	if err := db.Where("label =  ?", id).First(&sa).Error; err != nil {
		log.Errorf(ctx, "could not retrieve service account %s: %s", id, err.Error())
		return nil, err
	}

	return &sa, nil
}

func (manager SqlServiceAccountManager) ListOf(ctx context.Context, opts spellbook.ListOptions) ([]spellbook.Resource, error) {

	if current := spellbook.IdentityFromContext(ctx); current == nil || !current.HasPermission(spellbook.PermissionReadUser) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadUser))
	}

	var sas []*ServiceAccount
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
	if res := db.Find(&sas); res.Error != nil {
		log.Errorf(ctx, "error retrieving content: %s", res.Error.Error())
		return nil, res.Error
	}

	resources := make([]spellbook.Resource, len(sas))
	for i := range sas {
		resources[i] = sas[i]
	}
	return resources, nil
}

func (manager SqlServiceAccountManager) Patch(ctx context.Context, resource spellbook.Resource, fields map[string]interface{}) error {
	sa := resource.(*ServiceAccount)

	db := sql.FromContext(ctx)
	for k, v := range fields {
		if k == "token" {
			// delete a token
			if v == nil {
				sa.setToken("")
			} else {
				tkn := manager.tg.GenerateToken()
				sa.setToken(tkn)
			}
			return db.Save(sa).Error
		}
	}
	return spellbook.NewFieldError("", errors.New("specified field can't be patched"))
}

func (manager SqlServiceAccountManager) ListOfProperties(ctx context.Context, opts spellbook.ListOptions) ([]string, error) {
	if current := spellbook.IdentityFromContext(ctx); current == nil || !current.HasPermission(spellbook.PermissionReadUser) {
		return nil, spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionReadUser))
	}

	return nil, spellbook.NewUnsupportedError()
}

func (manager SqlServiceAccountManager) Create(ctx context.Context, res spellbook.Resource, bundle []byte) error {

	current := spellbook.IdentityFromContext(ctx)
	if current == nil || !current.HasPermission(spellbook.PermissionWriteUser) {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionWriteUser))
	}

	sa := res.(*ServiceAccount)

	uf := spellbook.NewRawField("label", true, sa.Label)
	uf.AddValidator(spellbook.DatastoreKeyNameValidator{})

	// validate the username. Accepted values for the username are implementation dependent
	if err := uf.Validate(); err != nil {
		msg := fmt.Sprintf("invalid label %s for service account", sa.Id())
		return spellbook.NewFieldError("label", errors.New(msg))
	}


	db := sql.FromContext(ctx)

	sa.Created = time.Now().UTC()
	if err := db.Create(sa).Error; err != nil {
		return fmt.Errorf("error creating service account %s: %s", sa.Label, err)
	}

	return nil
}

func (manager SqlServiceAccountManager) Update(ctx context.Context, res spellbook.Resource, bundle []byte) error {
	current := spellbook.IdentityFromContext(ctx)
	if !current.HasPermission(spellbook.PermissionWriteUser) {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionWriteUser))
	}

	o, _ := manager.NewResource(ctx)
	if err := o.FromRepresentation(spellbook.RepresentationTypeJSON, bundle); err != nil {
		return spellbook.NewFieldError("", fmt.Errorf("invalid json %s: %s", string(bundle), err.Error()))
	}

	other := o.(*ServiceAccount)
	sa := res.(*ServiceAccount)

	if !current.HasPermission(spellbook.PermissionEditPermissions) && sa.Permission != other.Permission {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionEditPermissions))
	}

	sa.Permission = other.Permission
	sa.Description = other.Description
	sa.IPRestrictions = other.IPRestrictions

	db := sql.FromContext(ctx)

	return db.Save(sa).Error
}

func (manager SqlServiceAccountManager) Delete(ctx context.Context, res spellbook.Resource) error {
	current := spellbook.IdentityFromContext(ctx)
	if !current.HasPermission(spellbook.PermissionWriteUser) {
		return spellbook.NewPermissionError(spellbook.PermissionName(spellbook.PermissionWriteUser))
	}

	sa := res.(*ServiceAccount)

	db := sql.FromContext(ctx)
	if err := db.Delete(&sa).Error; err != nil {
		return fmt.Errorf("error deleting service account %s: %s", sa.Label, err.Error())
	}

	return nil
}

