// Package cerr provides structured CLI errors with stable exit codes and
// JSON envelopes for programmatic consumption.
package cerr

import (
	"errors"
	"fmt"
)

// Kind categorises an error for routing and exit-code mapping.
type Kind int

// Error kinds.
const (
	KindAPI Kind = iota
	KindAuth
	KindValidation
	KindConfig
	KindIMAP
	KindClassify
	KindState
	KindDiscovery
	KindInternal
)

// String returns the lowercase tag used in JSON envelopes.
func (k Kind) String() string {
	switch k {
	case KindAPI:
		return "api"
	case KindAuth:
		return "auth"
	case KindValidation:
		return "validation"
	case KindConfig:
		return "config"
	case KindIMAP:
		return "imap"
	case KindClassify:
		return "classify"
	case KindState:
		return "state"
	case KindDiscovery:
		return "discovery"
	case KindInternal:
		return "internal"
	default:
		return "unknown"
	}
}

// Stable exit codes per Kind.
const (
	ExitCodeOK         = 0
	ExitCodeAPI        = 1
	ExitCodeAuth       = 2
	ExitCodeValidation = 3
	ExitCodeConfig     = 4
	ExitCodeIMAP       = 5
	ExitCodeClassify   = 6
	ExitCodeState      = 7
	ExitCodeDiscovery  = 8
	ExitCodeInternal   = 9
)

// ExitCodeDoc describes a single exit code for help/docs rendering.
type ExitCodeDoc struct {
	Code        int
	Description string
}

// ExitCodeDocs enumerates every exit code the CLI may emit, in stable order.
var ExitCodeDocs = []ExitCodeDoc{
	{ExitCodeOK, "success"},
	{ExitCodeAPI, "api error"},
	{ExitCodeAuth, "auth error"},
	{ExitCodeValidation, "validation error"},
	{ExitCodeConfig, "config error"},
	{ExitCodeIMAP, "imap error"},
	{ExitCodeClassify, "classify error"},
	{ExitCodeState, "state error"},
	{ExitCodeDiscovery, "discovery error"},
	{ExitCodeInternal, "internal error"},
}

// Error is a structured CLI error.
type Error struct {
	Kind    Kind
	Code    int
	Reason  string
	Message string
	Hint    string
	Cause   error
}

// Error implements the error interface.
func (e *Error) Error() string { return e.Message }

// Unwrap exposes the underlying cause for errors.Is / errors.As.
func (e *Error) Unwrap() error { return e.Cause }

// ExitCode maps Kind to a stable process exit code.
func (e *Error) ExitCode() int {
	switch e.Kind {
	case KindAPI:
		return ExitCodeAPI
	case KindAuth:
		return ExitCodeAuth
	case KindValidation:
		return ExitCodeValidation
	case KindConfig:
		return ExitCodeConfig
	case KindIMAP:
		return ExitCodeIMAP
	case KindClassify:
		return ExitCodeClassify
	case KindState:
		return ExitCodeState
	case KindDiscovery:
		return ExitCodeDiscovery
	case KindInternal:
		return ExitCodeInternal
	default:
		return ExitCodeInternal
	}
}

func format(msg string, args ...any) string {
	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}

// Validation builds a validation error.
func Validation(msg string, args ...any) *Error {
	return &Error{
		Kind:    KindValidation,
		Code:    400,
		Reason:  "validationError",
		Message: format(msg, args...),
	}
}

// Auth builds an auth error.
func Auth(msg string, args ...any) *Error {
	return &Error{
		Kind:    KindAuth,
		Code:    401,
		Reason:  "authError",
		Message: format(msg, args...),
	}
}

// IMAP builds an IMAP error wrapping cause.
func IMAP(cause error, msg string, args ...any) *Error {
	return &Error{
		Kind:    KindIMAP,
		Code:    502,
		Reason:  "imapError",
		Message: format(msg, args...),
		Cause:   cause,
	}
}

// Classify builds a classification error wrapping cause.
func Classify(cause error, msg string, args ...any) *Error {
	return &Error{
		Kind:    KindClassify,
		Code:    500,
		Reason:  "classifyError",
		Message: format(msg, args...),
		Cause:   cause,
	}
}

// State builds a state-store error wrapping cause.
func State(cause error, msg string, args ...any) *Error {
	return &Error{
		Kind:    KindState,
		Code:    500,
		Reason:  "stateError",
		Message: format(msg, args...),
		Cause:   cause,
	}
}

// Config builds a configuration error.
func Config(msg string, args ...any) *Error {
	return &Error{
		Kind:    KindConfig,
		Code:    400,
		Reason:  "configError",
		Message: format(msg, args...),
	}
}

// Discovery builds a discovery error wrapping cause.
func Discovery(cause error, msg string, args ...any) *Error {
	return &Error{
		Kind:    KindDiscovery,
		Code:    500,
		Reason:  "discoveryError",
		Message: format(msg, args...),
		Cause:   cause,
	}
}

// Internal builds an internal error wrapping cause.
func Internal(cause error, msg string, args ...any) *Error {
	return &Error{
		Kind:    KindInternal,
		Code:    500,
		Reason:  "internalError",
		Message: format(msg, args...),
		Cause:   cause,
	}
}

// API builds an API error with HTTP hint, reason, message and optional hint URL.
func API(httpCode int, reason, message, hint string) *Error {
	return &Error{
		Kind:    KindAPI,
		Code:    httpCode,
		Reason:  reason,
		Message: message,
		Hint:    hint,
	}
}

// From returns err as *Error. If err is already *Error it is returned as-is;
// otherwise it is wrapped as an internal error. Returns nil for nil.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return Internal(err, "%s", err.Error())
}
