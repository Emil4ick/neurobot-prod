package logging

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger создает и настраивает экземпляр логгера zap.
// Принимает окружение ('development' или 'production') и уровень логирования.
func NewLogger(environment string, level string) (*zap.Logger, error) {
	var loggerConfig zap.Config

	// Настройки для разных окружений
	if environment == "production" {
		loggerConfig = zap.NewProductionConfig()
        // В production можно писать в stdout/stderr (чтобы Docker/K8s собирали логи)
        // или в файл (требует настройки ротации логов)
        loggerConfig.OutputPaths = []string{"stdout"}
        loggerConfig.ErrorOutputPaths = []string{"stderr"}
	} else { // development и другие
		loggerConfig = zap.NewDevelopmentConfig()
        // В development удобно писать в stderr с цветом
        loggerConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
        loggerConfig.OutputPaths = []string{"stderr"}
        loggerConfig.ErrorOutputPaths = []string{"stderr"}
	}

	// Устанавливаем уровень логирования из конфига/аргумента
	logLevel := zapcore.InfoLevel // Уровень по умолчанию
	if err := logLevel.Set(level); err != nil {
		// Если уровень невалидный, используем InfoLevel и выводим предупреждение
		zap.S().Warnf("Неверный уровень логирования '%s', используется 'info'", level)
        logLevel = zapcore.InfoLevel
	}
	loggerConfig.Level = zap.NewAtomicLevelAt(logLevel)

    // Добавляем имя хоста и PID к записям (может быть полезно)
    hostname, _ := os.Hostname()
    pid := os.Getpid()
    loggerConfig.InitialFields = map[string]interface{}{
         "hostname": hostname,
         "pid": pid,
     }

	// Строим логгер
	logger, err := loggerConfig.Build()
	if err != nil {
		return nil, err
	}

    // Устанавливаем глобальный логгер (для удобства в некоторых случаях, но лучше передавать явно)
    // zap.ReplaceGlobals(logger)

	logger.Info("Логгер инициализирован", zap.String("environment", environment), zap.String("level", logLevel.String()))
	return logger, nil
}

// Пример получения уровня из конфига (может быть в другом месте)
// func GetLogLevelFromConfig(cfg *config.Config) string {
//     level := "info" // default
//     if cfg != nil && cfg.App.LogLevel != "" {
//          level = cfg.App.LogLevel
//     }
//     return level
// }