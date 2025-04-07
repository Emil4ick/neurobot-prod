package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"neurobot-prod/internal/config"
)

// NewRedisClient создает новый клиент Redis с оптимизациями для высокой нагрузки
func NewRedisClient(cfg config.RedisConfig, log *zap.Logger) (*redis.Client, error) {
	logger := log.Named("redis")

	// Создаем клиент Redis с оптимизированными настройками
	client := redis.NewClient(&redis.Options{
		Addr:            cfg.Addr,
		Password:        cfg.Password,
		DB:              cfg.DB,
		DialTimeout:     3 * time.Second,   // Максимальное время ожидания соединения
		ReadTimeout:     2 * time.Second,   // Максимальное время чтения
		WriteTimeout:    2 * time.Second,   // Максимальное время записи
		PoolSize:        50,                // Максимальное количество соединений в пуле
		MinIdleConns:    10,                // Минимальное количество простаивающих соединений
		MaxRetries:      3,                 // Количество повторных попыток при ошибке
		ConnMaxIdleTime: 240 * time.Second, // Время жизни неиспользуемого соединения
	})

	// Проверяем соединение
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := client.Ping(ctx).Result(); err != nil {
		return nil, fmt.Errorf("ошибка подключения к Redis: %w", err)
	}

	logger.Info("Подключение к Redis успешно установлено",
		zap.String("addr", cfg.Addr),
		zap.Int("db", cfg.DB),
		zap.Int("pool_size", 50),
		zap.Int("min_idle_conns", 10))

	return client, nil
}
