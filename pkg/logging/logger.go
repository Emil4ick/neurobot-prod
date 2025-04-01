package logging

import (
	"os"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger(environment string, level string) (*zap.Logger, error) {
	var loggerConfig zap.Config
	if environment == "production" {
		loggerConfig = zap.NewProductionConfig()
		loggerConfig.OutputPaths = []string{"stdout"}
		loggerConfig.ErrorOutputPaths = []string{"stderr"}
	} else {
		loggerConfig = zap.NewDevelopmentConfig()
		loggerConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		loggerConfig.OutputPaths = []string{"stderr"}
		loggerConfig.ErrorOutputPaths = []string{"stderr"}
	}
	logLevel := zapcore.InfoLevel
	if err := logLevel.Set(level); err != nil {
		zap.S().Warnf("Неверный уровень логирования '%s', используется 'info'", level)
		logLevel = zapcore.InfoLevel
	}
	loggerConfig.Level = zap.NewAtomicLevelAt(logLevel)
	hostname, _ := os.Hostname(); pid := os.Getpid()
	loggerConfig.InitialFields = map[string]interface{}{"hostname": hostname, "pid": pid}
	logger, err := loggerConfig.Build(zap.AddCallerSkip(1)) // Добавляем CallerSkip
	if err != nil { return nil, err }
	logger.Info("Логгер инициализирован", zap.String("environment", environment), zap.String("level", logLevel.String()))
	return logger, nil
}
