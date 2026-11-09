package log

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// disgo puts raw HTTP bodies and gateway frames under body and data, and the compiled
// request URL, which embeds interaction tokens and user IDs, under endpoint.
var droppedKeys = map[string]bool{"body": true, "data": true, "endpoint": true}

// disgo also formats whole payloads into error strings, which no key rule catches.
const maxAttrLen = 512

// disgo's exact log text for a gateway frame it skipped; only the first per process is raised to Error.
const droppedFrameMsg = "error while parsing gateway message"

var droppedFrameReported atomic.Bool

// Slog routes a library's slog output into zap, redacting raw payloads.
func Slog(logger *zap.Logger) *slog.Logger {
	return slog.New(&slogHandler{logger: logger})
}

type slogHandler struct {
	logger *zap.Logger
	prefix string
	attrs  []zap.Field
}

func (h *slogHandler) Enabled(_ context.Context, lvl slog.Level) bool {
	// Uncapped, so the record Handle raises to Error survives LOG_LEVEL=error; Handle caps the rest.
	return h.logger.Core().Enabled(zapLevel(lvl))
}

func (h *slogHandler) Handle(_ context.Context, r slog.Record) error {
	fields := make([]zap.Field, 0, len(h.attrs)+r.NumAttrs())
	fields = append(fields, h.attrs...)
	r.Attrs(func(a slog.Attr) bool {
		fields = appendAttr(fields, h.prefix, a)
		return true
	})
	lvl := bridgeLevel(r.Level)
	if r.Message == droppedFrameMsg && droppedFrameReported.CompareAndSwap(false, true) {
		lvl = zapcore.ErrorLevel
	}
	if ce := h.logger.Check(lvl, r.Message); ce != nil {
		ce.Write(fields...)
	}
	return nil
}

// Capped at Warn: disgo logs handled conditions such as gateway reconnects at Error, which
// would otherwise flood Sentry.
func bridgeLevel(lvl slog.Level) zapcore.Level {
	return min(zapLevel(lvl), zapcore.WarnLevel)
}

func (h *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := &slogHandler{
		logger: h.logger,
		prefix: h.prefix,
		attrs:  append([]zap.Field(nil), h.attrs...),
	}
	for _, a := range attrs {
		next.attrs = appendAttr(next.attrs, h.prefix, a)
	}
	return next
}

func (h *slogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &slogHandler{logger: h.logger, prefix: h.prefix + name + ".", attrs: h.attrs}
}

func appendAttr(fields []zap.Field, prefix string, a slog.Attr) []zap.Field {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return fields
	}

	if a.Value.Kind() == slog.KindGroup {
		group := a.Value.Group()
		inner := prefix
		if a.Key != "" {
			inner = prefix + a.Key + "."
		}
		for _, g := range group {
			fields = appendAttr(fields, inner, g)
		}
		return fields
	}

	if droppedKeys[a.Key] {
		return fields
	}
	// A stack trace holds frames, not payloads, and is useless cut short.
	if s := a.Value.String(); len(s) > maxAttrLen && a.Key != "stack" {
		elided := fmt.Sprintf("<elided %d bytes>", len(s))
		// disgo wraps decode errors around the raw frame and payload; only the innermost error is free of both.
		if err, ok := a.Value.Any().(error); ok {
			for next := errors.Unwrap(err); next != nil; next = errors.Unwrap(next) {
				err = next
			}
			if cause := err.Error(); len(cause) <= maxAttrLen {
				elided += ": " + cause
			}
		}
		return append(fields, zap.String(prefix+a.Key, elided))
	}
	return append(fields, zap.Any(prefix+a.Key, a.Value.Any()))
}

func zapLevel(lvl slog.Level) zapcore.Level {
	switch {
	case lvl < slog.LevelInfo:
		return zapcore.DebugLevel
	case lvl < slog.LevelWarn:
		return zapcore.InfoLevel
	case lvl < slog.LevelError:
		return zapcore.WarnLevel
	default:
		return zapcore.ErrorLevel
	}
}
