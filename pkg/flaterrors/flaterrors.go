package flaterrors

import (
	"unsafe"
)

type Unwrapper interface {
	Unwrap() []error
}

// Join returns an error that wraps the given errors.
// It flattens the provided errors if they can be unwrapped.
// Any nil error values are discarded.
// Join returns nil if every value in errs is nil.
// The error formats as the concatenation of the strings obtained
// by calling the Error method of each element of errs, with a newline
// between each string.
//
// A non-nil error returned by Join implements the Unwrap() []error method.
func Join(errs ...error) error {
	n := 0

	for _, err := range errs {
		if err != nil {
			n++
		}
	}

	if n == 0 {
		return nil
	}

	e := &joinError{
		errs: make([]error, 0, n),
	}

	for _, err := range errs {
		if err == nil {
			continue
		}

		if unwrapper, ok := err.(Unwrapper); ok {
			// NB: we do not check if err list contains nil values, because a flaterrors.Unwrapper must be sanitized
			// when initialized.
			e.errs = append(e.errs, unwrapper.Unwrap()...)

			continue
		}

		e.errs = append(e.errs, err)
	}

	return e
}

type joinError struct {
	errs []error
}

func (e *joinError) Error() string {
	// Since Join returns nil if every value in errs is nil,
	// e.errs cannot be empty.
	if len(e.errs) == 1 {
		return e.errs[0].Error()
	}

	b := []byte(e.errs[0].Error())
	for _, err := range e.errs[1:] {
		b = append(b, '\n')
		b = append(b, err.Error()...)
	}
	// At this point, b has at least one byte '\n'.
	return unsafe.String(&b[0], len(b))
}

func (e *joinError) Unwrap() []error {
	return e.errs
}
