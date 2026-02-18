package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type AppError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type HandledError struct {
	err error
}

func (e *HandledError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *HandledError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func NewError(code, message string, details map[string]any) *AppError {
	return &AppError{Code: code, Message: message, Details: details}
}

func MarkHandled(err error) error {
	if err == nil || IsHandled(err) {
		return err
	}
	return &HandledError{err: err}
}

func IsHandled(err error) bool {
	var handled *HandledError
	return errors.As(err, &handled)
}

func WriteJSONSuccess(w io.Writer, data any, meta map[string]any) error {
	resp := map[string]any{
		"ok":   true,
		"data": data,
	}
	if len(meta) > 0 {
		resp["meta"] = meta
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(resp)
}

func WriteJSONError(w io.Writer, appErr *AppError, meta map[string]any) error {
	if appErr == nil {
		appErr = NewError("internal_error", "internal error", nil)
	}
	resp := map[string]any{
		"ok":    false,
		"error": appErr,
	}
	if len(meta) > 0 {
		resp["meta"] = meta
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(resp)
}

func WriteHuman(w io.Writer, format string, args ...any) error {
	_, err := fmt.Fprintf(w, format+"\n", args...)
	return err
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case "validation_error":
			return 2
		case "not_found":
			return 3
		case "storage_error":
			return 10
		default:
			return 20
		}
	}

	return 20
}
