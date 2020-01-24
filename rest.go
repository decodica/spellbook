package spellbook

import (
	"cloud.google.com/go/datastore"
	"context"
	"decodica.com/flamel"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jinzhu/gorm"
	"google.golang.org/appengine/log"
	"net/http"
	"strconv"
	"strings"
)

type ReadHandler interface {
	HandleGet(context context.Context, key string, out *flamel.ResponseOutput) flamel.HttpResponse
}

type WriteHandler interface {
	HandlePost(context context.Context, out *flamel.ResponseOutput) flamel.HttpResponse
	HandlePut(context context.Context, key string, out *flamel.ResponseOutput) flamel.HttpResponse
	HandlePatch(context context.Context, key string, out *flamel.ResponseOutput) flamel.HttpResponse
	HandleDelete(context context.Context, key string, out *flamel.ResponseOutput) flamel.HttpResponse
}

type ListHandler interface {
	HandleList(context context.Context, out *flamel.ResponseOutput) flamel.HttpResponse
	HandlePropertyValues(context context.Context, out *flamel.ResponseOutput, property string) flamel.HttpResponse
}

type RestHandler interface {
	ReadHandler
	WriteHandler
	ListHandler
}

type BaseRestHandler struct {
	Manager Manager
}

// Builds the paging options, ordering and standard inputs of a given request
func (handler BaseRestHandler) buildOptions(ctx context.Context, out *flamel.ResponseOutput, opts *ListOptions) (*ListOptions, error) {
	// build paging
	opts.Size = 20
	opts.Page = 0

	ins := flamel.InputsFromContext(ctx)
	if pin, ok := ins["page"]; ok {
		if num, err := strconv.Atoi(pin.Value()); err == nil {
			if num > 0 {
				opts.Page = num
			}
		} else {
			msg := fmt.Sprintf("invalid page value : %v. page must be an integer", pin)
			return nil, errors.New(msg)
		}
	}

	if sin, ok := ins["results"]; ok {
		if num, err := strconv.Atoi(sin.Value()); err == nil {
			if num > 0 {
				opts.Size = num
			}
		} else {
			msg := fmt.Sprintf("invalid result size value : %v. results must be an integer", sin)
			return nil, errors.New(msg)
		}
	}

	// order is not mandatory
	if oin, ok := ins["order"]; ok {
		oins := oin.Value()
		// descendig has the format "-fieldname"
		opts.Descending = oins[:1] == "-"
		if opts.Descending {
			opts.Order = oins[1:]
		} else {
			opts.Order = oins
		}
	}

	// filter is not mandatory
	if fin, ok := ins["filter"]; ok {
		finv := fin.Value()
		filters := strings.Split(finv, "^")
		opts.Filters = make([]Filter, len(filters), cap(filters))
		for i, filter := range filters {
			farray := strings.Split(filter, "=")
			if len(farray) > 1 {
				opts.Filters[i] = Filter{farray[0], farray[1]}
			}
		}

	}

	return opts, nil
}

// REST Method handlers
func (handler BaseRestHandler) HandleGet(ctx context.Context, key string, out *flamel.ResponseOutput) flamel.HttpResponse {
	renderer := flamel.JSONRenderer{}
	out.Renderer = &renderer

	resource, err := handler.Manager.FromId(ctx, key)
	if err != nil {
		return handler.ErrorToStatus(ctx, err, out)
	}

	renderer.Data = resource
	return flamel.HttpResponse{Status: http.StatusOK}
}

// Called on GET requests.
// This handler is called when the available values of one property of a resource are requested
// Returns a list of the values that the requested property can assume
func (handler BaseRestHandler) HandlePropertyValues(ctx context.Context, out *flamel.ResponseOutput, prop string) flamel.HttpResponse {
	opts := &ListOptions{}
	opts.Property = prop
	opts, err := handler.buildOptions(ctx, out, opts)
	if err != nil {
		return flamel.HttpResponse{Status: http.StatusBadRequest}
	}

	results, err := handler.Manager.ListOfProperties(ctx, *opts)
	if err != nil {
		return handler.ErrorToStatus(ctx, err, out)
	}

	// output
	l := len(results)
	count := opts.Size
	if l < opts.Size {
		count = l
	}

	renderer := flamel.JSONRenderer{}
	renderer.Data = ListResponse{results[:count], l > opts.Size}

	out.Renderer = &renderer

	return flamel.HttpResponse{Status: http.StatusOK}
}

// Called on GET requests
// This handler is called when a list of resources is requested.
// Returns a paged result
func (handler BaseRestHandler) HandleList(ctx context.Context, out *flamel.ResponseOutput) flamel.HttpResponse {
	opts := &ListOptions{}
	opts, err := handler.buildOptions(ctx, out, opts)
	if err != nil {
		return flamel.HttpResponse{Status: http.StatusBadRequest}
	}

	results, err := handler.Manager.ListOf(ctx, *opts)
	if err != nil {
		return handler.ErrorToStatus(ctx, err, out)
	}

	// output
	l := len(results)
	count := opts.Size
	if l < opts.Size {
		count = l
	}

	var renderer flamel.Renderer

	// retrieve the negotiated method
	ins := flamel.InputsFromContext(ctx)
	accept := ins[flamel.KeyNegotiatedContent].Value()

	if accept == "text/csv" {
		r := &flamel.DownloadRenderer{}
		csv, err := Resources(results).ToCSV()
		if err != nil {
			return handler.ErrorToStatus(ctx, err, out)
		}
		r.Data = []byte(csv)
		renderer = r
	} else {
		jrenderer := flamel.JSONRenderer{}
		jrenderer.Data = ListResponse{results[:count], l > opts.Size}
		renderer = &jrenderer
	}

	out.Renderer = renderer

	return flamel.HttpResponse{Status: http.StatusOK}
}

// handles a POST request, ensuring the creation of the resource.
func (handler BaseRestHandler) HandlePost(ctx context.Context, out *flamel.ResponseOutput) flamel.HttpResponse {
	renderer := flamel.JSONRenderer{}
	out.Renderer = &renderer

	resource, err := handler.Manager.NewResource(ctx)
	if err != nil {
		return handler.ErrorToStatus(ctx, err, out)
	}

	errs := Errors{}
	// get the content data
	ins := flamel.InputsFromContext(ctx)
	j, ok := ins[flamel.KeyRequestJSON]
	if !ok {
		return flamel.HttpResponse{Status: http.StatusBadRequest}
	}

	err = resource.FromRepresentation(RepresentationTypeJSON, []byte(j.Value()))
	if err != nil {
		msg := fmt.Sprintf("bad json: %s", err.Error())
		errs.AddError("", errors.New(msg))
		log.Errorf(ctx, msg)
		renderer.Data = errs
		return flamel.HttpResponse{Status: http.StatusBadRequest}
	}

	if err = handler.Manager.Create(ctx, resource, []byte(j.Value())); err != nil {
		return handler.ErrorToStatus(ctx, err, out)
	}

	renderer.Data = resource
	return flamel.HttpResponse{Status: http.StatusCreated}
}

// Handles put requests, ensuring the update of the requested resource
func (handler BaseRestHandler) HandlePut(ctx context.Context, key string, out *flamel.ResponseOutput) flamel.HttpResponse {
	renderer := flamel.JSONRenderer{}
	out.Renderer = &renderer

	ins := flamel.InputsFromContext(ctx)
	j, ok := ins[flamel.KeyRequestJSON]
	if !ok {
		return flamel.HttpResponse{Status: http.StatusBadRequest}
	}

	resource, err := handler.Manager.FromId(ctx, key)
	if err != nil {
		return handler.ErrorToStatus(ctx, err, out)
	}

	if err = handler.Manager.Update(ctx, resource, []byte(j.Value())); err != nil {
		return handler.ErrorToStatus(ctx, err, out)
	}

	renderer.Data = resource
	return flamel.HttpResponse{Status: http.StatusOK}
}

// Handles put requests, ensuring the update of the requested resource
func (handler BaseRestHandler) HandlePatch(ctx context.Context, key string, out *flamel.ResponseOutput) flamel.HttpResponse {

	man, ok := handler.Manager.(PatchManager)
	if !ok {
		log.Debugf(ctx, "manager is %v", handler.Manager)
		return handler.ErrorToStatus(ctx, NewUnsupportedError(), out)
	}

	renderer := flamel.JSONRenderer{}
	out.Renderer = &renderer

	ins := flamel.InputsFromContext(ctx)
	j, ok := ins[flamel.KeyRequestJSON]
	if !ok {
		return flamel.HttpResponse{Status: http.StatusBadRequest}
	}

	resource, err := man.FromId(ctx, key)
	if err != nil {
		return handler.ErrorToStatus(ctx, err, out)
	}


	fields := map[string]interface{}{}
	if err := json.Unmarshal([]byte(j.Value()), &fields); err != nil {
		log.Errorf(ctx, "invalid json: %s", err)
		return handler.ErrorToStatus(ctx, NewFieldError("json", err), out)
	}

	if err = man.Patch(ctx, resource, fields); err != nil {
		return handler.ErrorToStatus(ctx, err, out)
	}


	renderer.Data = resource
	return flamel.HttpResponse{Status: http.StatusOK}
}

// Handles DELETE requests over a Resource type
func (handler BaseRestHandler) HandleDelete(ctx context.Context, key string, out *flamel.ResponseOutput) flamel.HttpResponse {
	renderer := flamel.JSONRenderer{}
	out.Renderer = &renderer

	resource, err := handler.Manager.FromId(ctx, key)
	if err != nil {
		return handler.ErrorToStatus(ctx, err, out)
	}

	if err = handler.Manager.Delete(ctx, resource); err != nil {
		return handler.ErrorToStatus(ctx, err, out)
	}
	return flamel.HttpResponse{Status: http.StatusOK}
}

// Converts an error to its equivalent HTTP representation
func (handler BaseRestHandler) ErrorToStatus(ctx context.Context, err error, out *flamel.ResponseOutput) flamel.HttpResponse {
	log.Errorf(ctx, "%s", err.Error())
	switch err.(type) {
	case UnsupportedError:
		return flamel.HttpResponse{Status: http.StatusMethodNotAllowed}
	case FieldError:
		renderer := flamel.JSONRenderer{}
		renderer.Data = struct {
			Field string
			Error string
		}{
			err.(FieldError).field,
			err.(FieldError).error.Error(),
		}
		out.Renderer = &renderer
		return flamel.HttpResponse{Status: http.StatusBadRequest}
	case PermissionError:
		renderer := flamel.JSONRenderer{}
		renderer.Data = struct {
			Error string
		}{
			err.(PermissionError).Error(),
		}
		out.Renderer = &renderer
		return flamel.HttpResponse{Status: http.StatusForbidden}
	default:
		if err == datastore.ErrNoSuchEntity {
			return flamel.HttpResponse{Status: http.StatusNotFound}
		}
		if err == gorm.ErrRecordNotFound {
			return flamel.HttpResponse{Status: http.StatusNotFound}
		}
		return flamel.HttpResponse{Status: http.StatusInternalServerError}
	}
}
