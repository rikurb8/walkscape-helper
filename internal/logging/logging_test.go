package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestInitVerbose(t *testing.T) {
	var buf bytes.Buffer
	Init(true, &buf)

	Debug("test debug message")
	Info("test info message")

	output := buf.String()
	if !strings.Contains(output, "test debug message") {
		t.Error("expected debug message in output")
	}
	if !strings.Contains(output, "test info message") {
		t.Error("expected info message in output")
	}
}

func TestInitNonVerbose(t *testing.T) {
	var buf bytes.Buffer
	Init(false, &buf)

	Debug("test debug message")
	Info("test info message")

	output := buf.String()
	if strings.Contains(output, "test debug message") {
		t.Error("expected debug message to be filtered in non-verbose mode")
	}
	if !strings.Contains(output, "test info message") {
		t.Error("expected info message in output")
	}
}

func TestWith(t *testing.T) {
	var buf bytes.Buffer
	Init(true, &buf)

	l := With("key", "value")
	l.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "key=value") {
		t.Error("expected key=value in output")
	}
}

func TestWithContext(t *testing.T) {
	var buf bytes.Buffer
	Init(true, &buf)

	ctx := context.Background()
	l := WithContext(ctx)
	if l == nil {
		t.Error("expected non-nil logger")
	}
}

func TestContextWithLogger(t *testing.T) {
	var buf bytes.Buffer
	Init(true, &buf)

	customLogger := slog.Default().With("custom", "value")
	ctx := ContextWithLogger(context.Background(), customLogger)

	l := WithContext(ctx)
	if l == nil {
		t.Error("expected non-nil logger from context")
	}
}

func TestWarnAndError(t *testing.T) {
	var buf bytes.Buffer
	Init(true, &buf)

	Warn("warning message")
	Error("error message")

	output := buf.String()
	if !strings.Contains(output, "warning message") {
		t.Error("expected warning message in output")
	}
	if !strings.Contains(output, "error message") {
		t.Error("expected error message in output")
	}
}
