package domain

import "errors"

// Error kinds classify why a use case was refused, independent of transport.
var (
	ErrNotFound    = errors.New("not found")
	ErrInvalid     = errors.New("invalid input")
	ErrForbidden   = errors.New("forbidden")
	ErrConflict    = errors.New("conflict")
	ErrUnsupported = errors.New("unsupported")
	ErrUnavailable = errors.New("unavailable")
	ErrUpstream    = errors.New("upstream failure")
	ErrTimeout     = errors.New("timed out")
	ErrInternal    = errors.New("internal failure")
)

// ErrLiveSyncFailed reports a persisted change that connected clients did not receive.
var ErrLiveSyncFailed = Unavailable("The change was saved, but live synchronization failed. Reload before retrying.")

// Error is a classified failure whose message is safe to show to users.
type Error struct {
	kind    error
	message string
	parent  *Error
	cause   error
}

func NotFound(message string) *Error    { return &Error{kind: ErrNotFound, message: message} }
func Invalid(message string) *Error     { return &Error{kind: ErrInvalid, message: message} }
func Forbidden(message string) *Error   { return &Error{kind: ErrForbidden, message: message} }
func Conflict(message string) *Error    { return &Error{kind: ErrConflict, message: message} }
func Unsupported(message string) *Error { return &Error{kind: ErrUnsupported, message: message} }
func Unavailable(message string) *Error { return &Error{kind: ErrUnavailable, message: message} }
func Upstream(message string) *Error    { return &Error{kind: ErrUpstream, message: message} }
func Timeout(message string) *Error     { return &Error{kind: ErrTimeout, message: message} }
func Internal(message string) *Error    { return &Error{kind: ErrInternal, message: message} }

func (e *Error) Error() string {
	return e.message
}

// Kind is the classification this error was created with, ignoring any cause it wraps.
func (e *Error) Kind() error {
	return e.kind
}

func (e *Error) Unwrap() []error {
	wrapped := []error{e.kind}
	if e.parent != nil {
		wrapped = append(wrapped, e.parent)
	}
	if e.cause != nil {
		wrapped = append(wrapped, e.cause)
	}
	return wrapped
}

// Detail extends the message while still matching e with errors.Is.
func (e *Error) Detail(detail string) *Error {
	return &Error{
		kind:    e.kind,
		message: e.message + ": " + detail,
		parent:  e,
	}
}

// WithCause keeps the underlying failure for diagnostics without changing the user message.
func (e *Error) WithCause(cause error) *Error {
	return &Error{
		kind:    e.kind,
		message: e.message,
		parent:  e,
		cause:   cause,
	}
}

// IsUserFacing reports whether err carries a message meant for users.
func IsUserFacing(err error) bool {
	var userErr *Error
	return errors.As(err, &userErr)
}
