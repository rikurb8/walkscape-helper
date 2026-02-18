package output

import (
	"errors"
	"testing"
)

func TestExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil", err: nil, want: 0},
		{name: "validation", err: NewError("validation_error", "bad", nil), want: 2},
		{name: "not_found", err: NewError("not_found", "missing", nil), want: 3},
		{name: "storage", err: NewError("storage_error", "db", nil), want: 10},
		{name: "unknown_app", err: NewError("unknown", "x", nil), want: 20},
		{name: "generic", err: errors.New("boom"), want: 20},
		{name: "handled_wrap", err: MarkHandled(NewError("validation_error", "bad", nil)), want: 2},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExitCode(tt.err)
			if got != tt.want {
				t.Fatalf("ExitCode() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestMarkHandled(t *testing.T) {
	t.Parallel()

	err := NewError("validation_error", "bad input", nil)
	wrapped := MarkHandled(err)
	if !IsHandled(wrapped) {
		t.Fatal("expected handled error")
	}

	var appErr *AppError
	if !errors.As(wrapped, &appErr) {
		t.Fatal("expected wrapped error to unwrap to AppError")
	}
	if appErr.Code != "validation_error" {
		t.Fatalf("unexpected code: %s", appErr.Code)
	}
}
