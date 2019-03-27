package errors

import (
	"fmt"
	"reflect"
)

type Error struct {
	// Cause error if any.
	parent error

	// This error scope description.
	desc string
}

func (e *Error) Cause() error {
	return e.parent
}

func (e *Error) Error() string {
	if e.parent == nil {
		return e.desc
	}
	return fmt.Sprintf("%s: %s", e.desc, e.parent)
}

// Is check if given error instance is of a given kind/type. This involves
// unwrapping given error using the Cause method if available.
func (kind *Error) Is(err error) bool {
	// Reflect usage is necessary to correctly compare with
	// a nil implementation of an error.
	if kind == nil {
		if err == nil {
			return true
		}
		return reflect.ValueOf(err).IsNil()
	}

	for {
		if err == kind {
			return true
		}

		if c, ok := err.(causer); ok {
			err = c.Cause()
		} else {
			return false
		}
	}
}

// Wrap extends given error with an additional information.
//
// If the wrapped error does not provide ABCICode method (ie. stdlib errors),
// it will be labeled as internal error.
//
// If err is nil, this returns nil, avoiding the need for an if statement when
// wrapping a error returned at the end of a function
func Wrap(err error, description string) *Error {
	if err == nil {
		return nil
	}
	return &Error{
		parent: err,
		desc:   description,
	}
}

// Wrapf extends given error with an additional information.
//
// This function works like Wrap function with additional funtionality of
// formatting the input as specified.
func Wrapf(err error, format string, args ...interface{}) *Error {
	desc := fmt.Sprintf(format, args...)
	return Wrap(err, desc)
}

// New returns a new error instance that does not have a parent. Returned
// instance is a root cause error.
func New(description string) *Error {
	return &Error{
		parent: nil,
		desc:   description,
	}
}

// causer is an interface implemented by an error that supports wrapping. Use
// it to test if an error wraps another error instance.
type causer interface {
	Cause() error
}
