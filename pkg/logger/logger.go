package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	instance *zap.Logger
	sugar    *zap.SugaredLogger
	once     sync.Once
)

// Init initialises the global logger. Must be called before any logging.
func Init(env string) {
	once.Do(func() {
		var cfg zap.Config
		if env == "production" {
			cfg = zap.NewProductionConfig()
			cfg.EncoderConfig.TimeKey = "timestamp"
			cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		} else {
			cfg = zap.NewDevelopmentConfig()
			cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		}

		l, err := cfg.Build(
			zap.AddCaller(),
			zap.AddCallerSkip(1),
		)
		if err != nil {
			// Fallback to stderr
			l, _ = zap.NewProduction()
		}

		instance = l
		sugar = l.Sugar()
	})
}

// get returns the logger, initialising with development config if needed.
func get() *zap.Logger {
	if instance == nil {
		Init("development")
	}
	return instance
}

// getSugar returns the sugared logger.
func getSugar() *zap.SugaredLogger {
	if sugar == nil {
		Init("development")
	}
	return sugar
}

// Named returns a named child logger.
func Named(name string) *zap.Logger { return get().Named(name) }

// With returns a logger with extra fields.
func With(fields ...zap.Field) *zap.Logger { return get().With(fields...) }

// Info logs at INFO level.
func Info(msg string, fields ...zap.Field) { get().Info(msg, fields...) }

// Error logs at ERROR level.
func Error(msg string, fields ...zap.Field) { get().Error(msg, fields...) }

// Warn logs at WARN level.
func Warn(msg string, fields ...zap.Field) { get().Warn(msg, fields...) }

// Debug logs at DEBUG level.
func Debug(msg string, fields ...zap.Field) { get().Debug(msg, fields...) }

// Fatal logs at FATAL level and then calls os.Exit(1).
func Fatal(msg string, fields ...zap.Field) { get().Fatal(msg, fields...) }

// Panic logs at PANIC level and then panics.
func Panic(msg string, fields ...zap.Field) { get().Panic(msg, fields...) }

// Infof logs a formatted message at INFO level.
func Infof(template string, args ...interface{}) { getSugar().Infof(template, args...) }

// Errorf logs a formatted message at ERROR level.
func Errorf(template string, args ...interface{}) { getSugar().Errorf(template, args...) }

// Warnf logs a formatted message at WARN level.
func Warnf(template string, args ...interface{}) { getSugar().Warnf(template, args...) }

// Debugf logs a formatted message at DEBUG level.
func Debugf(template string, args ...interface{}) { getSugar().Debugf(template, args...) }

// Fatalf logs a formatted message at FATAL level and calls os.Exit(1).
func Fatalf(template string, args ...interface{}) { getSugar().Fatalf(template, args...) }

// Sync flushes any buffered log entries.
func Sync() {
	if instance != nil {
		_ = instance.Sync()
	}
}

// GetZap returns the underlying *zap.Logger for use with third-party integrations.
func GetZap() *zap.Logger { return get() }

// NewRequestLogger creates a logger that writes request logs to stdout in JSON.
func NewRequestLogger(env string) *zap.Logger {
	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var enc zapcore.Encoder
	if env == "production" {
		enc = zapcore.NewJSONEncoder(encoderCfg)
	} else {
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		enc = zapcore.NewConsoleEncoder(encoderCfg)
	}

	core := zapcore.NewCore(enc, zapcore.AddSync(os.Stdout), zapcore.InfoLevel)
	return zap.New(core)
}
