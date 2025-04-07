package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"go.uber.org/zap"

	"neurobot-prod/internal/config"
)

// NewPostgresDB создает новое подключение к базе данных PostgreSQL с оптимизациями для высокой нагрузки
func NewPostgresDB(cfg config.DBConfig, log *zap.Logger) (*sql.DB, error) {
	logger := log.Named("postgres")

	// Создаем строку подключения
	connStr := cfg.ConnectionString()

	// Открываем соединение с базой данных
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия соединения с базой данных: %w", err)
	}

	// Оптимизация пула соединений для высокой нагрузки
	// Увеличиваем максимальное количество открытых соединений
	db.SetMaxOpenConns(100) // Оптимальное значение зависит от аппаратных ресурсов

	// Увеличиваем количество простаивающих соединений
	db.SetMaxIdleConns(25)

	// Устанавливаем время жизни соединения
	db.SetConnMaxLifetime(time.Hour)

	// Устанавливаем максимальное время простоя соединения
	db.SetConnMaxIdleTime(30 * time.Minute)

	// Проверяем соединение
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ошибка при проверке соединения с базой данных: %w", err)
	}

	logger.Info("Подключение к PostgreSQL успешно установлено",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("database", cfg.Name),
		zap.Int("max_open_conns", 100),
		zap.Int("max_idle_conns", 25),
		zap.Duration("conn_max_lifetime", time.Hour),
		zap.Duration("conn_max_idle_time", 30*time.Minute))

	return db, nil
}
