package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"neurobot-prod/internal/app"
	"neurobot-prod/internal/config"
	"neurobot-prod/pkg/logging"
)

func main() {
	// Парсим флаги командной строки
	configPath := flag.String("config", "./config", "Путь к каталогу с конфигурацией")
	mode := flag.String("mode", "webhook", "Режим работы бота: webhook, worker или llm")
	flag.Parse()

	// Инициализируем логгер
	logger, err := logging.NewLogger("development", "info")
	if err != nil {
		fmt.Printf("Ошибка инициализации логгера: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Загружаем конфигурацию
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logger.Fatal("Ошибка загрузки конфигурации", zap.Error(err))
	}

	// Обновляем настройки логгера
	logger, err = logging.NewLogger(cfg.App.Env, cfg.App.LogLevel)
	if err != nil {
		fmt.Printf("Ошибка обновления логгера: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Запуск приложения в режиме", zap.String("mode", *mode))

	// Работаем в зависимости от выбранного режима
	switch *mode {
	case "webhook":
		// Запускаем вебхук-сервер
		runWebhookServer(cfg, logger)

	case "worker":
		// Запускаем обработчик сообщений
		runMessageWorker(cfg, logger)

	case "llm":
		// Запускаем LLM-воркер
		runLLMWorker(cfg, logger)

	default:
		logger.Fatal("Неизвестный режим работы", zap.String("mode", *mode))
	}
}

// runWebhookServer запускает сервер для обработки вебхуков
func runWebhookServer(cfg *config.Config, logger *zap.Logger) {
	// Создаем обработчик вебхуков
	webhookHandler := app.NewWebhookHandler(cfg, logger)

	// Запускаем сервер
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Services.Webhook.Port),
		Handler: webhookHandler.Router(),
	}

	// Запускаем HTTP сервер в отдельной горутине
	go func() {
		logger.Info("Запуск webhook-сервера",
			zap.String("address", server.Addr),
			zap.String("webhook_path", cfg.Telegram.WebhookPath))

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Ошибка запуска webhook-сервера", zap.Error(err))
		}
	}()

	// Настраиваем канал для обработки сигналов остановки
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Ожидаем сигнал
	sig := <-quit
	logger.Info("Получен сигнал остановки", zap.String("signal", sig.String()))

	// Задаем таймаут для graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Останавливаем сервер
	logger.Info("Останавливаем webhook-сервер...")
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Ошибка при остановке webhook-сервера", zap.Error(err))
	}

	logger.Info("Webhook-сервер остановлен")
}

// runMessageWorker запускает обработчик сообщений
func runMessageWorker(cfg *config.Config, logger *zap.Logger) {
	// Создаем обработчик сообщений
	messageWorker, err := app.NewMessageWorker(cfg, logger)
	if err != nil {
		logger.Fatal("Ошибка создания обработчика сообщений", zap.Error(err))
	}

	// Запускаем обработчик сообщений
	err = messageWorker.Start()
	if err != nil {
		logger.Fatal("Ошибка запуска обработчика сообщений", zap.Error(err))
	}

	logger.Info("Обработчик сообщений запущен")

	// Настраиваем канал для обработки сигналов остановки
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Ожидаем сигнал
	sig := <-quit
	logger.Info("Получен сигнал остановки", zap.String("signal", sig.String()))

	// Останавливаем обработчик сообщений
	messageWorker.Stop()

	logger.Info("Обработчик сообщений остановлен")
}

// runLLMWorker запускает обработчик LLM-запросов
func runLLMWorker(cfg *config.Config, logger *zap.Logger) {
	// Создаем обработчик LLM-запросов
	llmWorker, err := app.NewLLMWorker(cfg, logger)
	if err != nil {
		logger.Fatal("Ошибка создания обработчика LLM-запросов", zap.Error(err))
	}

	// Запускаем обработчик LLM-запросов
	err = llmWorker.Start()
	if err != nil {
		logger.Fatal("Ошибка запуска обработчика LLM-запросов", zap.Error(err))
	}

	logger.Info("LLM-обработчик запущен")

	// Настраиваем канал для обработки сигналов остановки
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Ожидаем сигнал
	sig := <-quit
	logger.Info("Получен сигнал остановки", zap.String("signal", sig.String()))

	// Останавливаем обработчик LLM-запросов
	llmWorker.Stop()

	logger.Info("LLM-обработчик остановлен")
}
