package ipc

import (
	"context"
	"errors"
	"fmt"
	"regexp"
)

// Stable invoke codes. A page switches on these; the message is for humans.
const (
	CodeCanceled             = "ipc.canceled"
	CodeTimeout              = "ipc.timeout"
	CodeInvalidTimeout       = "ipc.invalid_timeout"
	CodeUnknownCommand       = "ipc.unknown_command"
	CodeMissingCmd           = "ipc.missing_cmd"
	CodeInvalidMessage       = "ipc.invalid_message"
	CodeNoRouter             = "ipc.no_router"
	CodePanic                = "ipc.panic"
	CodeBadResponse          = "ipc.bad_response"
	CodeUnsupported          = "ipc.unsupported"
	CodeMissingChannel       = "ipc.missing_channel"
	CodeChannelClosed        = "ipc.channel_closed"
	CodeChannelArgs          = "ipc.channel_args"
	CodeChannelCount         = "ipc.channel_count"
	CodeChannelsUnconfigured = "ipc.channels_unconfigured"
	CodeEncode               = "ipc.encode"
	CodeFailed               = "command.failed"
)

// ErrCanceled is returned when the page cancels an invoke, or the frame drops it.
var ErrCanceled = fail(CodeCanceled, "ipc: canceled")

// ErrTimeout is returned when an invoke exceeds its deadline.
var ErrTimeout = fail(CodeTimeout, "ipc: timeout")

// ErrInvalidTimeout is returned when the page asks for a timeout this host rejects.
var ErrInvalidTimeout = fail(CodeInvalidTimeout, "ipc: invalid timeout")

// codePattern is lowercase segments separated by dots, at least two segments.
var codePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)

const maxCodeLen = 64

// Error is a command failure the page can switch on.
type Error struct {
	code    string
	message string
}

// Fail returns an invoke error with a stable code.
// code is lowercase segments separated by dots, such as fs.denied.
// An invalid code is reported as command.failed. The message is Error().
func Fail(code, format string, args ...any) error {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	return fail(code, msg)
}

func fail(code, message string) *Error {
	if !validCode(code) {
		code = CodeFailed
	}
	if message == "" {
		message = "invoke failed"
	}
	return &Error{code: code, message: message}
}

func validCode(code string) bool {
	return len(code) <= maxCodeLen && codePattern.MatchString(code)
}

// Error is the text the page shows.
func (e *Error) Error() string {
	if e == nil || e.message == "" {
		return "invoke failed"
	}
	return e.message
}

// Code is the stable token. Invalid codes are not constructed by Fail.
func (e *Error) Code() string {
	if e == nil {
		return ""
	}
	return e.code
}

// Is reports whether target is an Error with the same code.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok || e == nil || t == nil {
		return false
	}
	return e.code != "" && e.code == t.code
}

type coder interface {
	error
	Code() string
}

// CodeOf reports the stable code carried by err.
// A plain error has no code. Context cancellation and deadlines are mapped
// by Describe, not here.
func CodeOf(err error) string {
	var c coder
	if err == nil || !errors.As(err, &c) {
		return ""
	}
	code := c.Code()
	if !validCode(code) {
		return ""
	}
	return code
}

// Describe is the code and message written into a failed invoke response.
// Uncoded errors use command.failed and err.Error() as the message.
func Describe(err error) (code, message string) {
	if err == nil {
		return CodeFailed, "invoke failed"
	}
	if c := CodeOf(err); c != "" {
		msg := err.Error()
		if msg == "" {
			msg = "invoke failed"
		}
		return c, msg
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return CodeTimeout, ErrTimeout.Error()
	case errors.Is(err, context.Canceled):
		return CodeCanceled, ErrCanceled.Error()
	}
	msg := err.Error()
	if msg == "" {
		msg = "invoke failed"
	}
	return CodeFailed, msg
}
