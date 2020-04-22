package spellbook

import (
	"context"
	"decodica.com/flamel"
	"golang.org/x/text/language"
	"sync"
)

var once sync.Once
var instance *Website

type Website struct {
	flamel.Application
	Router  InternationalRouter
	options Options
}

//singleton instance
func Application() *Website {

	once.Do(func() {
		instance = &Website{}
	})

	return instance
}

func (app Website) OnStart(ctx context.Context) context.Context {
	return ctx
}

func (app Website) AfterResponse(ctx context.Context) {}

func (app *Website) SetOptions(opts Options) {
	app.options = opts
}

func (app Website) Options() Options {
	return app.options
}

func (app Website) SupportsLocale(val string) bool {
	for _, v := range app.options.Languages {
		if v.String() == val {
			return true
		}
	}
	return false
}

type DefaultAttachmentGroup struct {
	Name        string
	Type        string
	MaxItem     int
	Description string
}

type SupportedCategory struct {
	Name                    string
	Label                   string
	Type                    string
	DefaultAttachmentGroups []DefaultAttachmentGroup
}

type StaticPageCode string
type SpecialCode string

type ActionType string

const (
	ActionTypeNormal = "normal"
	ActionTypeUpload = "upload"
)

type SupportedAction struct {
	Name     string
	Endpoint string
	Type     ActionType
	Method   string
}

type Options struct {
	// application GCS bucket
	Bucket string
	// password salt.
	Salt         string
	Languages    []language.Tag
	Categories   []SupportedCategory
	StaticPages  []StaticPageCode
	SpecialCodes []SpecialCode
	Actions      []SupportedAction
}

func NewWebsite(opts *Options) *Website {
	ws := Application()
	if opts.Languages != nil {
		// create the language matcher
		ws.Router = NewInternationalRouter()
		ws.Router.matcher = language.NewMatcher(opts.Languages)
	}
	ws.SetOptions(*opts)
	return ws
}
