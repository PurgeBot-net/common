package log

import (
	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

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
	return cfg.Build()
}

func WithSentry(logger *zap.Logger, dsn string) *zap.Logger {
	if dsn == "" {
		return logger
	}
	_ = sentry.Init(sentry.ClientOptions{Dsn: dsn})
	return logger.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return zapcore.NewTee(core, &sentryCore{})
	}))
}

type sentryCore struct{}

func (s *sentryCore) Enabled(lvl zapcore.Level) bool      { return lvl >= zapcore.ErrorLevel }
func (s *sentryCore) With(_ []zapcore.Field) zapcore.Core { return s }
func (s *sentryCore) Sync() error                         { return nil }
func (s *sentryCore) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if s.Enabled(e.Level) {
		return ce.AddCore(e, s)
	}
	return ce
}
func (s *sentryCore) Write(e zapcore.Entry, _ []zapcore.Field) error {
	sentry.CaptureMessage(e.Message)
	return nil
}
