package page

import (
	"context"
	"distudio.com/mage"
	"fmt"
	"golang.org/x/text/language"
)

const KeyLanguageParam = "lang"
const KeyLanguageTag = "__p_languageTag__"

type InternationalRouter struct {
	*mage.DefaultRouter
	matcher language.Matcher
}

func NewInternationalRouter() InternationalRouter {
	router := InternationalRouter{}
	router.DefaultRouter = mage.NewDefaultRouter()
	return router
}

func (router *InternationalRouter) SetRoutes(urls []string, handler func(ctx context.Context) mage.Controller, authenticator mage.Authenticator) {
	for _, v := range urls {
		router.SetRoute(v, handler, authenticator)
	}
}

func (router *InternationalRouter) SetUniversalRoute(url string, handler func(ctx context.Context) mage.Controller, authenticator mage.Authenticator) {
	router.DefaultRouter.SetRoute(url, handler, authenticator)
}

func (router *InternationalRouter) SetRoute(url string, handler func(ctx context.Context) mage.Controller, authenticator mage.Authenticator) {

	// if no language is specified, redirect to the default language
	router.Router.SetRoute(url, func(ctx context.Context) (interface{}, context.Context) {
		lang,_, _ := router.matcher.Match(language.Make(""))
		url := fmt.Sprintf("/%s%s", lang.String(), url)
		return &RedirectController{To: url}, ctx
	})

	// prepend the url with the language param
	lurl := fmt.Sprintf("/:%s%s", KeyLanguageParam, url)
	// add the language-corrected route to the router
	router.Router.SetRoute(lurl, func(ctx context.Context) (interface{}, context.Context) {
		if authenticator != nil {
			ctx = authenticator.Authenticate(ctx)
		}
		// add the language tag to the route, if supported
		params := mage.RoutingParams(ctx)
		lkey := params[KeyLanguageParam].Value()
		lang := language.Make(lkey)
		tag, _, _ := router.matcher.Match(lang)
		if t := tag.String(); lkey != t {
			url := fmt.Sprintf("/%s%s",t, url)
			return &RedirectController{To:url}, ctx
		}
		ctx = context.WithValue(ctx, KeyLanguageTag, tag)
		return handler(ctx), ctx
	})
}

func (router *InternationalRouter) RouteForPath(ctx context.Context, path string) (context.Context, error, mage.Controller) {
	c, err, controller := router.Router.RouteForPath(ctx, path)
	if err != nil {
		return c, err, nil
	}
	return c, nil, controller.(mage.Controller)
}