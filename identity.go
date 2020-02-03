package spellbook

import "context"

type Permission int64

const (
	HeaderToken string = "X-Authentication"
	keyUser     string = "__pUser__"
)

const (
	PermissionEnabled = 1 << iota
	PermissionEditPermissions
	PermissionReadUser
	PermissionWriteUser
	PermissionReadContent
	PermissionWriteContent
	PermissionReadMailMessage
	PermissionWriteMailMessage
	PermissionReadPlace
	PermissionWritePlace
	PermissionReadMedia
	PermissionWriteMedia
	PermissionReadPage
	PermissionWritePage
	PermissionWriteSubscription
	PermissionReadSubscription
	PermissionWriteAction
	PermissionReadAction
)

var Permissions = map[Permission]string{
	PermissionEnabled:           "PERMISSION_ENABLED",
	PermissionEditPermissions:   "PERMISSION_EDIT_PERMISSIONS",
	PermissionReadUser:          "PERMISSION_READ_USER",
	PermissionWriteUser:         "PERMISSION_WRITE_USER",
	PermissionReadContent:       "PERMISSION_READ_CONTENT",
	PermissionWriteContent:      "PERMISSION_WRITE_CONTENT",
	PermissionReadMailMessage:   "PERMISSION_READ_MAILMESSAGE",
	PermissionWriteMailMessage:  "PERMISSION_WRITE_MAILMESSAGE",
	PermissionReadPlace:         "PERMISSION_READ_PLACE",
	PermissionWritePlace:        "PERMISSION_WRITE_PLACE",
	PermissionReadMedia:         "PERMISSION_READ_MEDIA",
	PermissionWriteMedia:        "PERMISSION_WRITE_MEDIA",
	PermissionReadPage:          "PERMISSION_READ_PAGE",
	PermissionWritePage:         "PERMISSION_WRITE_PAGE",
	PermissionReadSubscription:  "PERMISSION_READ_SUBSCRIPTION",
	PermissionWriteSubscription: "PERMISSION_WRITE_SUBSCRIPTION",
	PermissionWriteAction: "PERMISSION_READ_ACTION",
	PermissionReadAction: "PERMISSION_WRITE_ACTION",
}

func PermissionName(permission Permission) string {
	return Permissions[permission]
}

func NamedPermissionToPermission(name string) Permission {
	for permission, n := range Permissions {
		if n == name {
			return permission
		}
	}
	return Permission(0)
}

type Identity interface {
	HasPermission(permission Permission) bool
	Username() string
}

func IdentityFromContext(ctx context.Context) Identity {
	if id := ctx.Value(keyUser); id != nil {
		return id.(Identity)
	}
	return nil
}

func ContextWithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, keyUser, id)
}
