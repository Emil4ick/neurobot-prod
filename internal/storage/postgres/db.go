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

// NewPostgresDB создает новое подключение к базе данных PostgreSQL
func NewPostgresDB(cfg config.DBConfig, log *zap.Logger) (*sql.DB, error) {
	logger := log.Named("postgres")

	// Создаем строку подключения
	connStr := cfg.ConnectionString()

	// Открываем соединение с базой данных
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия соединения с базой данных: %w", err)
	}

	// Устанавливаем параметры пула соединений
	db.SetMaxOpenConns(int(cfg.PoolMaxConns))
	db.SetMaxIdleConns(int(cfg.PoolMinConns))
	db.SetConnMaxLifetime(time.Hour)

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
		zap.String("database", cfg.Name))

	return db, nil
}
