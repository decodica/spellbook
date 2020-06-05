package identity

import (
	"context"
	"decodica.com/flamel"
	"decodica.com/spellbook"
	"github.com/decodica/model/v2"
	"google.golang.org/appengine/user"
)

type UserAuthenticator struct {
	flamel.Authenticator
}

func (authenticator UserAuthenticator) Authenticate(ctx context.Context) context.Context {
	inputs := flamel.InputsFromContext(ctx)
	if tkn, ok := inputs[spellbook.HeaderToken]; ok {
		token := tkn.Value()
		// grab the last chars after hashLength
		encoded := token[hashLen:]
		u := User{}
		err := model.FromEncodedKey(ctx, &u, encoded)
		if err != nil || u.Token != token {
			return ctx
		}

		if !u.IsEnabled() {
			return ctx
		}

		return spellbook.ContextWithIdentity(ctx, u)
	}

	return ctx
}

type GSupportAuthenticator struct {
	flamel.Authenticator
}

func (authenticator GSupportAuthenticator) Authenticate(ctx context.Context) context.Context {
	guser := user.Current(ctx)
	if guser == nil {
		// try with the canonical authenticator
		ua := UserAuthenticator{}
		return ua.Authenticate(ctx)
	}

	// else populate a mage user with usable fields
	u := User{}
	u.gUser = guser
	u.Email = guser.Email
	// if admin, grant all permissions
	u.GrantAll()
	return spellbook.ContextWithIdentity(ctx, u)
}
