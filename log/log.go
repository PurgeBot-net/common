package log

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const sentryFlushTimeout = 2 * time.Second

// Promoted to Sentry tags, which are filterable; every other field becomes context.
var sentryTagKeys = []string{"job_id", "guild_id", "channel_id", "purge_type", "target_type"}

type Config struct {
	SentryDSN string `env:"SENTRY_DSN"`
	Level     string `env:"LOG_LEVEL" envDefault:"info"`
	JSON      bool   `env:"LOG_JSON"`
}

// Snowflakes exceed 2^53, so a JSON reader using float64 corrupts them as numbers.
func ID(key string, id uint64) zap.Field {
	return zap.String(key, strconv.FormatUint(id, 10))
}

func New(level string, json bool) (*zap.Logger, error) {
	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = zapcore.InfoLevel
	}

	var cfg zap.Config
	if json {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
	}
	cfg.Level = zap.NewAtomicLevelAt(lvl)
	// The console preset is a development config, which zap reads as "stacktrace from Warn
	// up". Options passed to Build are applied after cfg.buildOptions, so this overrides it.
	return cfg.Build(zap.AddStacktrace(zapcore.ErrorLevel))
}

func WithSentry(logger *zap.Logger, dsn string) *zap.Logger {
	if dsn == "" {
		return logger
	}
	if err := sentry.Init(sentry.ClientOptions{Dsn: dsn}); err != nil {
		logger.Error("init sentry, error reporting disabled", zap.Error(err))
		return logger
	}
	return logger.WithOptions(
		zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			return zapcore.NewTee(core, &sentryCore{})
		}),
		// Fatal calls os.Exit, so the deferred Sync that would otherwise flush never runs.
		zap.WithFatalHook(flushThenExit{}),
	)
}

type flushThenExit struct{}

func (flushThenExit) OnWrite(*zapcore.CheckedEntry, []zapcore.Field) {
	sentry.Flush(sentryFlushTimeout)
	os.Exit(1)
}

type sentryCore struct {
	fields []zapcore.Field
}

func (s *sentryCore) Enabled(lvl zapcore.Level) bool { return lvl >= zapcore.ErrorLevel }

func (s *sentryCore) With(fields []zapcore.Field) zapcore.Core {
	return &sentryCore{fields: append(slices.Clip(s.fields), fields...)}
}

func (s *sentryCore) Sync() error {
	sentry.Flush(sentryFlushTimeout)
	return nil
}

func (s *sentryCore) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if s.Enabled(e.Level) {
		return ce.AddCore(e, s)
	}
	return ce
}

func (s *sentryCore) Write(e zapcore.Entry, fields []zapcore.Field) error {
	tags, extra, cause := splitFields(append(slices.Clip(s.fields), fields...))

	// Cloned per event: Write is concurrent and mutates the scope.
	hub := sentry.CurrentHub().Clone()
	scope := hub.Scope()
	scope.SetLevel(sentryLevel(e.Level))
	scope.SetTags(tags)
	if len(extra) > 0 {
		scope.SetContext("log", extra)
	}
	if cause == nil {
		hub.CaptureMessage(e.Message)
		return nil
	}
	// Wrapped so issues group on the message too, not just the error type.
	hub.CaptureException(fmt.Errorf("%s: %w", e.Message, cause))
	return nil
}

// An error field encodes to its string form, so the value is taken off the raw field
// to survive as an exception. NamedError keeps its own key and stays context.
func splitFields(fields []zapcore.Field) (map[string]string, sentry.Context, error) {
	var cause error
	enc := zapcore.NewMapObjectEncoder()
	for _, f := range fields {
		if f.Type == zapcore.ErrorType && f.Key == "error" && cause == nil {
			if err, ok := f.Interface.(error); ok {
				cause = err
				continue
			}
		}
		f.AddTo(enc)
	}

	tags := make(map[string]string)
	extra := make(sentry.Context, len(enc.Fields))
	for k, v := range enc.Fields {
		if slices.Contains(sentryTagKeys, k) {
			tags[k] = fmt.Sprint(v)
			continue
		}
		extra[k] = v
	}
	return tags, extra, cause
}

// Only Error and above reach Write, so nothing maps to a lower Sentry level.
func sentryLevel(lvl zapcore.Level) sentry.Level {
	if lvl >= zapcore.PanicLevel {
		return sentry.LevelFatal
	}
	return sentry.LevelError
}
