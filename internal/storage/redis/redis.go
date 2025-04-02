package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"neurobot-prod/internal/config"
)

// NewRedisClient создает новый клиент Redis
func NewRedisClient(cfg config.RedisConfig, log *zap.Logger) (*redis.Client, error) {
	logger := log.Named("redis")

	// Создаем клиент Redis
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Проверяем соединение
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := client.Ping(ctx).Result(); err != nil {
		return nil, fmt.Errorf("ошибка подключения к Redis: %w", err)
	}

	logger.Info("Подключение к Redis успешно установлено",
		zap.String("addr", cfg.Addr),
		zap.Int("db", cfg.DB))

	return client, nil
}
