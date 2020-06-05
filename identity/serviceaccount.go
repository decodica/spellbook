package identity

import (
	"database/sql"
	"decodica.com/spellbook"
	"encoding/json"
	"github.com/decodica/model/v2"
	"time"
)

const SAIdentifier = "SA:"

func IsServiceAccountToken(tkn string) bool {
	return tkn[:3] == SAIdentifier
}

type ServiceAccount struct {
	model.Model    `json:"-"`
	Label          string `gorm:"PRIMARY_KEY;"`
	Description    string
	Token          string               `gorm:"-"`
	SqlToken       sql.NullString       `gorm:"UNIQUE_INDEX:idx_serviceaccount_token;column:token"`
	IPRestrictions string               // comma-separated strings for ip restrictions
	Permission     spellbook.Permission `gorm:"NOT NULL"`
	Created        time.Time
}

func (sa *ServiceAccount) setToken(tkn string) {
	sa.Token = tkn
	sa.SqlToken.Valid = tkn != ""
	sa.SqlToken.String = tkn
}

func (sa *ServiceAccount) getToken() string {
	if sa.SqlToken.Valid {
		return sa.SqlToken.String
	}
	return sa.Token
}

func (sa *ServiceAccount) UnmarshalJSON(data []byte) error {
	// username (alias StringID) must be handled by the consumer of the model
	alias := struct {
		Label          string   `json:"label"`
		Description    string   `json:"description"`
		IPRestrictions string   `json:"ipRestrictions"`
		Permissions    []string `json:"permissions"`
	}{}

	err := json.Unmarshal(data, &alias)
	if err != nil {
		return err
	}

	sa.Label = alias.Label
	sa.Description = alias.Description
	sa.IPRestrictions = alias.IPRestrictions
	sa.GrantNamedPermissions(alias.Permissions)
	return nil
}

func (sa *ServiceAccount) MarshalJSON() ([]byte, error) {
	type Alias struct {
		Label          string   `json:"label"`
		Description    string   `json:"description"`
		Token          string   `json:"token"`
		IPRestrictions string   `json:"ipRestrictions"`
		Permissions    []string `json:"permissions"`
	}

	return json.Marshal(&struct {
		Username string `json:"username"`
		Alias
	}{
		sa.Label,
		Alias{
			Label:          sa.Label,
			Description:    sa.Description,
			Token:          sa.getToken(),
			IPRestrictions: sa.IPRestrictions,
			Permissions:    sa.Permissions(),
		},
	})
}

func (sa ServiceAccount) Permissions() []string {
	var perms []string
	for permission, description := range spellbook.Permissions {
		if sa.HasPermission(permission) {
			perms = append(perms, description)
		}
	}
	return perms
}

func (sa ServiceAccount) HasPermission(permission spellbook.Permission) bool {
	return sa.Permission&permission != 0
}

func (sa *ServiceAccount) GrantNamedPermissions(names []string) {
	for _, name := range names {
		sa.GrantNamedPermission(name)
	}
}

func (sa *ServiceAccount) GrantNamedPermission(name string) {
	permission := spellbook.NamedPermissionToPermission(name)
	sa.GrantPermission(permission)
}

func (sa *ServiceAccount) GrantPermission(permission spellbook.Permission) {
	sa.Permission |= permission
}

func (sa *ServiceAccount) IsEnabled() bool {
	return sa.HasPermission(spellbook.PermissionEnabled)
}

/**
-- Resource implementation
*/

func (sa *ServiceAccount) Id() string {
	if sa.EncodedKey() == "" {
		return sa.Label
	}
	return sa.StringID()
}

func (sa *ServiceAccount) FromRepresentation(rtype spellbook.RepresentationType, data []byte) error {
	switch rtype {
	case spellbook.RepresentationTypeJSON:
		return json.Unmarshal(data, sa)
	}
	return spellbook.NewUnsupportedError()
}

func (sa *ServiceAccount) ToRepresentation(rtype spellbook.RepresentationType) ([]byte, error) {
	switch rtype {
	case spellbook.RepresentationTypeJSON:
		return json.Marshal(sa)
	}
	return nil, spellbook.NewUnsupportedError()
}
func (sa ServiceAccount) Username() string {
	return sa.Label
}
