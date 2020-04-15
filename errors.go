package spellbook

import (
	"encoding/json"
	"errors"
	"fmt"
)

var ErrMissingField = errors.New("missing field")

// Errors is a response object that contains the list of possible request errors
type Errors struct {
	errors []FieldError
}

func (errs Errors) HasErrors() bool {
	return errs.errors != nil
}

func (errs *Errors) AddFieldError(error FieldError) {
	errs.errors = append(errs.errors, error)
}

func (errs *Errors) AddError(name string, value error) {
	errs.errors = append(errs.errors, NewFieldError(name, value))
}

func (errs *Errors) Clear() {
	errs.errors = nil
}

func (errs Errors) MarshalJSON() ([]byte, error) {
	return json.Marshal(errs.errors)
}

// FieldError represents a missing required field or a malformed field.
// It usually leads to a BadRequest Error
// arguments is a slice containing possible error variables
type FieldError struct {
	error
	field string
	args []string
}

func NewFieldError(field string, error error) FieldError {
	return FieldError{error: error, field: field}
}

func (err *FieldError) AddArgument(argument string) {
	err.args = append(err.args, argument)
}

func (err FieldError) MarshalJSON() ([]byte, error) {
	type Alias struct {
		Field string `json:"field"`
		Error string `json:"error"`
		Args []string `json:"args"`
	}
	return json.Marshal(Alias{err.field, err.Error(), err.args})
}

// Permission error denotes that the requested action cannot be performed
// because the requestor lacks the required permission to do so
type PermissionError struct {
	permission string
}

func (err PermissionError) Error() string {
	return fmt.Sprintf("missing permission: %s", err.permission)
}

func NewPermissionError(permission string) PermissionError {
	return PermissionError{permission}
}

// Unsupported error is used to notify that the action requested is not supported
type UnsupportedError struct{}

func (err UnsupportedError) Error() string {
	return fmt.Sprint("action is not supported")
}

func NewUnsupportedError() UnsupportedError {
	return UnsupportedError{}
}
