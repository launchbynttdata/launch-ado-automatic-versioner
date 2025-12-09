package logging

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	LevelTerse   = "terse"
	LevelVerbose = "verbose"
)

// New creates a zap logger configured for the requested verbosity level.
func New(level string) (*zap.Logger, error) {
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.TimeKey = "time"
	encoderCfg.LevelKey = "level"
	encoderCfg.MessageKey = "msg"

	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(zapcore.InfoLevel),
		Encoding:         "console",
		EncoderConfig:    encoderCfg,
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	switch level {
	case LevelVerbose:
		cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case LevelTerse, "":
		cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	default:
		return nil, fmt.Errorf("unknown log level %q", level)
	}

	return cfg.Build()
}
