package spellbook

import (
	"context"
	"decodica.com/flamel"
	"fmt"
	"golang.org/x/text/language"
	"net/http"
	"strings"
)

const KeyLanguageParam = "lang"
const KeyLanguageTag = "__p_languageTag__"

type InternationalRouter struct {
	*flamel.DefaultRouter
	matcher language.Matcher
}

func NewInternationalRouter() InternationalRouter {
	router := InternationalRouter{}
	router.DefaultRouter = flamel.NewDefaultRouter()
	return router
}

func (router InternationalRouter) SetRoutes(urls []string, handler func(ctx context.Context) flamel.Controller, authenticator flamel.Authenticator) {
	for _, v := range urls {
		router.SetRoute(v, handler, authenticator)
	}
}

func (router InternationalRouter) SetUniversalRoute(url string, handler func(ctx context.Context) flamel.Controller, authenticator flamel.Authenticator) {
	router.DefaultRouter.SetRoute(url, handler, authenticator)
}

func (router InternationalRouter) SetUniversalRoutes(urls []string, handler func(ctx context.Context) flamel.Controller, authenticator flamel.Authenticator) {
	for _, v := range urls {
		router.SetUniversalRoute(v, handler, authenticator)
	}
}

func (router InternationalRouter) SetRoute(url string, handler func(ctx context.Context) flamel.Controller, authenticator flamel.Authenticator) {

	// if no language is specified, redirect to the default language
	router.Router.SetRoute(url, func(ctx context.Context) (interface{}, context.Context) {
		lang, _, _ := router.matcher.Match(language.Make(""))
		parms := flamel.InputsFromContext(ctx)
		url := parms[flamel.KeyRequestURL].Value()
		url = fmt.Sprintf("/%s%s", lang.String(), url)
		switch parms[flamel.KeyRequestMethod].Value() {
		case http.MethodGet:
			if query, ok := parms[flamel.KeyRequestQuery]; ok && query.Value() != "" {
				url = fmt.Sprintf("%s?%s", url, query.Value())
			}
			fallthrough
		case http.MethodHead:
			return &RedirectController{To: url}, ctx
		default:
			return &TemporaryRedirectController{To: url}, ctx
		}
	})

	for _, l := range Application().Options().Languages {
		// else a language has been specified, prepend the url with the language param
		lurl := fmt.Sprintf("/%s%s", l.String(), url)
		// add the language-corrected route to the router
		router.Router.SetRoute(lurl, func(ctx context.Context) (interface{}, context.Context) {
			if authenticator != nil {
				ctx = authenticator.Authenticate(ctx)
			}
			// add the language tag to the route, if supported
			idx := strings.Index(lurl[1:], "/")
			lkey := lurl[1 : idx+1]
			lang := language.Make(lkey)
			tag, _, _ := router.matcher.Match(lang)
			if t := tag.String(); lkey != t {
				url := fmt.Sprintf("/%s%s", t, url)
				// if its not a get request, return a 307
				parms := flamel.InputsFromContext(ctx)
				switch parms[flamel.KeyRequestMethod].Value() {
				case http.MethodGet:
					fallthrough
				case http.MethodHead:
					return &RedirectController{To: url}, ctx
				default:
					return &TemporaryRedirectController{To: url}, ctx
				}

			}
			ctx = context.WithValue(ctx, KeyLanguageTag, tag)
			return handler(ctx), ctx
		})
	}
}

func (router InternationalRouter) RouteForPath(ctx context.Context, path string) (context.Context, error, flamel.Controller) {
	c, err, controller := router.Router.RouteForPath(ctx, path)
	if err != nil {
		return c, err, nil
	}
	return c, nil, controller.(flamel.Controller)
}

func LocaleStringFromContext(ctx context.Context) string {
	tag, ok := ctx.Value(KeyLanguageTag).(language.Tag)
	if !ok {
		return ""
	}

	return tag.String()
}
