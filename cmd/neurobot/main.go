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
	mode := flag.String("mode", "webhook", "Режим работы бота: webhook или polling")
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
	defer messageWorker.Stop()

	// Создаем и запускаем webhook-сервер для бота
	var webhookServer *http.Server
	if *mode == "webhook" {
		// Создаем обработчик webhook
		webhookHandler := app.NewWebhookHandler(cfg, logger)

		// Запускаем сервер webhook
		webhookServer = &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.Services.Webhook.Port),
			Handler: webhookHandler.Router(),
		}

		// Запускаем HTTP сервер в отдельной горутине
		go func() {
			logger.Info("Запуск webhook-сервера",
				zap.String("address", webhookServer.Addr),
				zap.String("webhook_path", cfg.Telegram.WebhookPath))

			if err := webhookServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Fatal("Ошибка запуска webhook-сервера", zap.Error(err))
			}
		}()
	}

	// Настраиваем канал для обработки сигналов остановки
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Ожидаем сигнал
	sig := <-quit
	logger.Info("Получен сигнал остановки", zap.String("signal", sig.String()))

	// Задаем таймаут для graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Останавливаем webhook-сервер если он был запущен
	if webhookServer != nil {
		logger.Info("Останавливаем webhook-сервер...")
		if err := webhookServer.Shutdown(ctx); err != nil {
			logger.Error("Ошибка при остановке webhook-сервера", zap.Error(err))
		}
	}

	logger.Info("Приложение завершило работу")
}
