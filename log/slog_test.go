package log

import (
	"errors"
	"log/slog"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func observedSlog() (*slog.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.DebugLevel)
	return Slog(zap.New(core)), logs
}

func TestSlogDropsBodyAttr(t *testing.T) {
	logger, logs := observedSlog()
	logger.Debug("new response",
		slog.String("endpoint", "/channels/1/messages"),
		slog.String("body", "the message content"),
	)

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %#v", entries)
	}
	ctx := entries[0].ContextMap()
	if _, ok := ctx["body"]; ok {
		t.Fatalf("body attribute reached zap: %#v", ctx)
	}
	if ctx["endpoint"] != "/channels/1/messages" {
		t.Fatalf("endpoint attribute lost: %#v", ctx)
	}
}

func TestSlogDropsDataAttrFromWithAttrs(t *testing.T) {
	logger, logs := observedSlog()
	logger.With(slog.String("data", "a raw gateway frame")).Debug("received gateway message")

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %#v", entries)
	}
	if ctx := entries[0].ContextMap(); len(ctx) != 0 {
		t.Fatalf("data attribute survived With: %#v", ctx)
	}
}

func TestSlogElidesOversizedAttr(t *testing.T) {
	logger, logs := observedSlog()
	payload := strings.Repeat("a", maxAttrLen+1)
	logger.Error("error while parsing gateway message", slog.Any("err", errors.New(payload)))

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %#v", entries)
	}
	if entries[0].Level != zapcore.ErrorLevel {
		t.Fatalf("want error level, got %#v", entries[0])
	}
	got := entries[0].ContextMap()["err"]
	if strings.Contains(got.(string), "aaa") {
		t.Fatalf("payload survived elision: %#v", got)
	}
	if got != "<elided 513 bytes>" {
		t.Fatalf("want elision marker, got %#v", got)
	}
}

func TestSlogKeepsShortAttr(t *testing.T) {
	logger, logs := observedSlog()
	logger.Warn("rate limit exceeded", slog.Any("err", errors.New("429 too many requests")))

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %#v", entries)
	}
	if got := entries[0].ContextMap()["err"]; got != "429 too many requests" {
		t.Fatalf("short error was not passed through: %#v", got)
	}
}
