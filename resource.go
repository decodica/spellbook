package spellbook

import (
	"context"
	"decodica.com/flamel"
	"decodica.com/spellbook/format/csv"
	"errors"
	"google.golang.org/appengine/log"
	"net/http"
)

type ListOptions struct {
	Size       int
	Page       int
	Order      string // field
	Descending bool   // if -Order = desc
	Property   string
	Filters    []Filter // example url: &filter=Locale=it^Category=services
}

type FilterOperator string
const (
	FilterOperatorLessThan = "lt"
	FilterOperatorGreaterThan = "gt"
	FilterOperatorLessOrEqualThan = "le"
	FilterOperatorGreaterOrEqualThan = "ge"
	FilterOperatorExact = "ex"
	FilterOperatorLike = "li"
)

type Filter struct {
	Field string
	Value string
	Operator FilterOperator
}

type ListResponse struct {
	Items interface{} `json:"items"`
	More  bool        `json:"more"`
}

type Manager interface {
	NewResource(ctx context.Context) (Resource, error)
	FromId(ctx context.Context, id string) (Resource, error)
	ListOf(ctx context.Context, opts ListOptions) ([]Resource, error)
	ListOfProperties(ctx context.Context, opts ListOptions) ([]string, error)
	Create(ctx context.Context, resource Resource, bundle []byte) error
	Update(ctx context.Context, resource Resource, bundle []byte) error
	Delete(ctx context.Context, resource Resource) error
}

type PatchManager interface {
	Manager
	Patch(ctx context.Context, resource Resource, fields map[string]interface{}) error
}

type RepresentationType int

const (
	RepresentationTypeJSON = iota
	RepresentationTypeUrlencoded
	RepresentationTypeCSV
)

type Resource interface {
	Id() string
	ToRepresentation(rtype RepresentationType) ([]byte, error)
	FromRepresentation(rtype RepresentationType, data []byte) error
}

type Resources []Resource

func (r Resources) ToCSV() (string, error) {
	csvbles := make([]csv.CSVble, len(r))
	for i, res := range r {
		csvble, ok := res.(csv.CSVble)
		if !ok {
			return "", NewFieldError("", errors.New("resource is not convertible to csv"))
		}
		csvbles[i] = csvble
	}
	return csv.MarshalList(csvbles)
}

func (r Resources) FromCSV([]string) error {
	return NewUnsupportedError()
}

/**
* Base rest controller
 */

type RestController struct {
	Key     string
	Private bool
	RestHandler
	extenders map[string][]Extender
}

const FilterPropertyKey = "property"
const FilterOrderKey = "order"
const FilterResultsKey = "results"
const FilterPageKey = "page"

func NewBaseRestController() *RestController {
	return NewRestController(BaseRestHandler{})
}

func NewRestController(handler RestHandler) *RestController {
	return &RestController{RestHandler: handler}
}

func (controller *RestController) AddExtender(hook string, extender Extender) {
	e, ok := controller.extenders[hook]
	if !ok {
		e = make([]Extender, 0)
	}
	e = append(e, extender)
	controller.extenders[hook] = e
}

func (controller *RestController) Process(ctx context.Context, out *flamel.ResponseOutput) flamel.HttpResponse {

	u := IdentityFromContext(ctx)

	if controller.Private && u == nil {
		return flamel.HttpResponse{Status: http.StatusUnauthorized}
	}

	ins := flamel.InputsFromContext(ctx)

	method := ins[flamel.KeyRequestMethod].Value()
	hasKey := controller.Key != ""
	prop, hasProperty := ins[FilterPropertyKey]

	switch method {
	case http.MethodPost:
		return controller.HandlePost(ctx, out)
	case http.MethodGet:
		if !hasKey {
			if hasProperty {
				return controller.HandlePropertyValues(ctx, out, prop.Value())
			}
			return controller.HandleList(ctx, out)
		}
		return controller.HandleGet(ctx, controller.Key, out)
	case http.MethodPatch:
		if !hasKey {
			log.Errorf(ctx, "no item was specify for patch method")
			return flamel.HttpResponse{Status: http.StatusBadRequest}
		}
		return controller.HandlePatch(ctx, controller.Key, out)
	case http.MethodPut:
		if !hasKey {
			log.Errorf(ctx, "no item was specify for put method")
			return flamel.HttpResponse{Status: http.StatusBadRequest}
		}
		return controller.HandlePut(ctx, controller.Key, out)
	case http.MethodDelete:
		if !hasKey {
			log.Errorf(ctx, "no item was specify for delete method")
			return flamel.HttpResponse{Status: http.StatusBadRequest}
		}
		return controller.HandleDelete(ctx, controller.Key, out)
	}

	return flamel.HttpResponse{Status: http.StatusNotImplemented}
}

func (controller *RestController) OnDestroy(ctx context.Context) {}
