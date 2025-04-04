package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// AppConfig содержит общие настройки приложения
type AppConfig struct {
	Env      string `mapstructure:"env"`
	LogLevel string `mapstructure:"log_level"`
	Domain   string `mapstructure:"domain"` // Домен для вебхуков и Mini App
}

// NATSConfig содержит настройки для NATS
type NATSConfig struct {
	URL                  string       `mapstructure:"url"`
	ReconnectWaitSeconds int          `mapstructure:"reconnect_wait_seconds"`
	MaxReconnects        int          `mapstructure:"max_reconnects"`
	TimeoutSeconds       int          `mapstructure:"timeout_seconds"`
	Subjects             NATSSubjects `mapstructure:"subjects"`
}

// NATSSubjects содержит темы для NATS
type NATSSubjects struct {
	TelegramUpdates string `mapstructure:"telegram_updates"`
	LLMTasks        string `mapstructure:"llm_tasks"`
	LLMResults      string `mapstructure:"llm_results"`
}

// TelegramConfig содержит настройки для Telegram API
type TelegramConfig struct {
	WebhookPath    string `mapstructure:"webhook_path"` // Путь для вебхука
	WebhookBaseURL string `mapstructure:"webhook_base_url"`
	WebAppURL      string `mapstructure:"webapp_url"` // URL для Mini App
	Token          string // Заполняется из ENV
	SecretToken    string // Заполняется из ENV
}

// DBConfig содержит настройки для базы данных
type DBConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Name         string `mapstructure:"name"`
	SSLMode      string `mapstructure:"sslmode"`
	PoolMinConns int32  `mapstructure:"pool_min_conns"`
	PoolMaxConns int32  `mapstructure:"pool_max_conns"`
	Password     string // Заполняется из ENV
}

// RedisConfig содержит настройки для Redis
type RedisConfig struct {
	Addr              string `mapstructure:"addr"`
	DB                int    `mapstructure:"db"`
	DefaultTTLSeconds int    `mapstructure:"default_ttl_seconds"`
	CacheTTLSeconds   int    `mapstructure:"cache_ttl_seconds"`
	Password          string // Заполняется из ENV
}

// Config содержит полную конфигурацию приложения
type Config struct {
	App      AppConfig
	NATS     NATSConfig
	Telegram TelegramConfig
	DB       DBConfig
	Redis    RedisConfig
	// Другие поля конфигурации...
}

// GetReconnectWait возвращает время ожидания переподключения
func (c NATSConfig) GetReconnectWait() time.Duration {
	return time.Duration(c.ReconnectWaitSeconds) * time.Second
}

// GetTimeout возвращает время ожидания таймаута
func (c NATSConfig) GetTimeout() time.Duration {
	return time.Duration(c.TimeoutSeconds) * time.Second
}

// LoadConfig загружает конфигурацию из файла и переменных окружения
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	// Устанавливаем значения по умолчанию
	setDefaults(v)

	v.AddConfigPath(configPath)
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("ошибка чтения файла конфигурации: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("не удалось разобрать конфигурацию: %w", err)
	}

	// Читаем секреты из переменных окружения
	cfg.Telegram.Token = v.GetString("telegram.token")
	cfg.Telegram.SecretToken = v.GetString("telegram.secret_token")
	cfg.DB.Password = v.GetString("db.password")
	cfg.Redis.Password = v.GetString("redis.password")

	return &cfg, nil
}

// setDefaults устанавливает значения по умолчанию
func setDefaults(v *viper.Viper) {
	// App
	v.SetDefault("app.env", "development")
	v.SetDefault("app.log_level", "info")
	v.SetDefault("app.domain", "yourneuro.ru")

	// NATS
	v.SetDefault("nats.url", "nats://localhost:4222")
	v.SetDefault("nats.reconnect_wait_seconds", 2)
	v.SetDefault("nats.max_reconnects", 60)
	v.SetDefault("nats.timeout_seconds", 1)
	v.SetDefault("nats.subjects.telegram_updates", "tg.updates.v1")
	v.SetDefault("nats.subjects.llm_tasks", "llm.tasks.v1")
	v.SetDefault("nats.subjects.llm_results", "llm.results.v1")

	// Telegram
	v.SetDefault("telegram.webhook_path", "/webhook")
	v.SetDefault("telegram.webhook_base_url", "https://yourneuro.ru")
	v.SetDefault("telegram.webapp_url", "https://yourneuro.ru/webapp")

	// DB
	v.SetDefault("db.host", "localhost")
	v.SetDefault("db.port", 5432)
	v.SetDefault("db.user", "neuro_user")
	v.SetDefault("db.name", "neurobot_db")
	v.SetDefault("db.sslmode", "disable")
	v.SetDefault("db.pool_min_conns", 2)
	v.SetDefault("db.pool_max_conns", 10)

	// Redis
	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.default_ttl_seconds", 600)
	v.SetDefault("redis.cache_ttl_seconds", 3600) // 1 час для кэша
}

// ConnectionString генерирует строку подключения к PostgreSQL
func (c *DBConfig) ConnectionString() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s&pool_min_conns=%d&pool_max_conns=%d",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode, c.PoolMinConns, c.PoolMaxConns)
}
