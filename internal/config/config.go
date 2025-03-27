package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config - корневая структура конфигурации приложения.
// Теги `mapstructure` помогают Viper правильно связать ключи YAML/env с полями Go.
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Telegram TelegramConfig `mapstructure:"telegram"`
	DB       DBConfig       `mapstructure:"db"`
	Redis    RedisConfig    `mapstructure:"redis"`
	NATS     NATSConfig     `mapstructure:"nats"`
	OpenAI   OpenAIConfig   `mapstructure:"openai"`
	Services ServiceConfig  `mapstructure:"services"`
}

type AppConfig struct {
	Env string `mapstructure:"env"`
}

type TelegramConfig struct {
	Token           string `mapstructure:"token"`
	WebhookBaseURL  string `mapstructure:"webhook_base_url"`
	WebhookPath     string `mapstructure:"webhook_path"`
	SecretToken     string `mapstructure:"secret_token"`
}

// Полный URL вебхука для установки в Telegram
func (t TelegramConfig) GetWebhookURL() string {
    // Убираем лишние слеши при соединении
    return strings.TrimSuffix(t.WebhookBaseURL, "/") + "/" + strings.TrimPrefix(t.WebhookPath, "/")
}

type DBConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	Name         string `mapstructure:"name"`
	SSLMode      string `mapstructure:"sslmode"`
	PoolMinConns int32  `mapstructure:"pool_min_conns"` // Используем int32 для совместимости с pgxpool
	PoolMaxConns int32  `mapstructure:"pool_max_conns"` // Используем int32 для совместимости с pgxpool
}

// ConnectionString формирует DSN (Data Source Name) для подключения к PostgreSQL через pgx.
func (c *DBConfig) ConnectionString() string {
	// postgres://user:password@host:port/dbname?sslmode=disable&pool_min_conns=2&pool_max_conns=15
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s&pool_min_conns=%d&pool_max_conns=%d",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode, c.PoolMinConns, c.PoolMaxConns)
}

type RedisConfig struct {
	Addr              string `mapstructure:"addr"`
	Password          string `mapstructure:"password"`
	DB                int    `mapstructure:"db"`
	DefaultTTLSeconds int    `mapstructure:"default_ttl_seconds"`
}

func (c RedisConfig) GetDefaultTTL() time.Duration {
	return time.Duration(c.DefaultTTLSeconds) * time.Second
}

type NATSConfig struct {
	URL                 string        `mapstructure:"url"`
	ReconnectWaitSeconds int          `mapstructure:"reconnect_wait_seconds"`
	MaxReconnects       int          `mapstructure:"max_reconnects"`
	TimeoutSeconds      int          `mapstructure:"timeout_seconds"`
	Subjects            NATSSubjects  `mapstructure:"subjects"`
}
type NATSSubjects struct {
	TelegramUpdates string `mapstructure:"telegram_updates"`
	LLMTasks        string `mapstructure:"llm_tasks"`
	LLMResults      string `mapstructure:"llm_results"`
}

func (c NATSConfig) GetReconnectWait() time.Duration {
	return time.Duration(c.ReconnectWaitSeconds) * time.Second
}
func (c NATSConfig) GetTimeout() time.Duration {
	return time.Duration(c.TimeoutSeconds) * time.Second
}

type OpenAIConfig struct {
	ApiKey string `mapstructure:"api_key"`
	Model  string `mapstructure:"model"`
}

type ServiceConfig struct {
	Webhook WebhookServiceConfig `mapstructure:"webhook"`
}
type WebhookServiceConfig struct {
	Port int `mapstructure:"port"`
}

// LoadConfig загружает конфигурацию из файла и/или переменных окружения.
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	// 1. Устанавливаем значения по умолчанию (для случая отсутствия файла/env)
	setDefaults(v)

	// 2. Настраиваем чтение из файла (если он есть)
	v.AddConfigPath(configPath) // Путь к папке с конфигом
	v.SetConfigName("config")   // Имя файла без расширения
	v.SetConfigType("yaml")     // Тип файла

	// Пытаемся прочитать файл, но не считаем ошибкой, если он не найден
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Если ошибка НЕ "файл не найден", то это проблема
			return nil, fmt.Errorf("ошибка чтения файла конфигурации (%s): %w", v.ConfigFileUsed(), err)
		}
		// Файл не найден - это нормально, полагаемся на defaults и env
	}

	// 3. Настраиваем чтение из переменных окружения
	v.AutomaticEnv() // Автоматически читать переменные окружения
	// Заменяем точки на подчеркивания (e.g., db.host -> DB_HOST)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
    // Можно установить префикс, например "NB_" (NB_DB_HOST)
    // v.SetEnvPrefix("NB")

	// 4. Анмаршалим прочитанные значения в структуру Config
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("не удалось разобрать конфигурацию: %w", err)
	}

	// 5. Валидация критичных параметров (пример)
	if err := validateConfig(cfg); err != nil {
         return nil, fmt.Errorf("ошибка валидации конфигурации: %w", err)
    }

	return &cfg, nil
}

// setDefaults устанавливает значения по умолчанию для Viper.
func setDefaults(v *viper.Viper) {
    v.SetDefault("app.env", "development")

	v.SetDefault("telegram.webhook_base_url", "http://localhost:8080")
	v.SetDefault("telegram.webhook_path", "/webhook")

	v.SetDefault("db.host", "localhost")
	v.SetDefault("db.port", 5432)
	v.SetDefault("db.user", "neuro_user")
	v.SetDefault("db.password", "neuro_secret_pass")
	v.SetDefault("db.name", "neurobot_db")
	v.SetDefault("db.sslmode", "disable")
	v.SetDefault("db.pool_min_conns", 2)
	v.SetDefault("db.pool_max_conns", 10) // Небольшой пул по умолчанию

	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
    v.SetDefault("redis.default_ttl_seconds", 600) // 10 минут

	v.SetDefault("nats.url", "nats://localhost:4222")
    v.SetDefault("nats.reconnect_wait_seconds", 2)
    v.SetDefault("nats.max_reconnects", 60)
    v.SetDefault("nats.timeout_seconds", 1)
	v.SetDefault("nats.subjects.telegram_updates", "tg.updates.v1")
	v.SetDefault("nats.subjects.llm_tasks", "llm.tasks.v1")
    v.SetDefault("nats.subjects.llm_results", "llm.results.v1")

	v.SetDefault("openai.model", "gpt-3.5-turbo")

	v.SetDefault("services.webhook.port", 8080)
}

// validateConfig проверяет наличие и корректность критичных параметров.
func validateConfig(cfg Config) error {
    if cfg.Telegram.Token == "" || cfg.Telegram.Token == "YOUR_TELEGRAM_BOT_TOKEN" {
        // В production окружении это должна быть фатальная ошибка
        fmt.Fprintln(os.Stderr, "ПРЕДУПРЕЖДЕНИЕ: TELEGRAM_TOKEN не установлен!")
        // return errors.New("TELEGRAM_TOKEN is required")
    }
     if cfg.Telegram.SecretToken == "" || cfg.Telegram.SecretToken == "a_very_secret_string_12345" {
         fmt.Fprintln(os.Stderr, "ПРЕДУПРЕЖДЕНИЕ: TELEGRAM_SECRET_TOKEN не установлен или используется значение по умолчанию!")
         // return errors.New("TELEGRAM_SECRET_TOKEN is required and should be strong")
    }
     if cfg.OpenAI.ApiKey == "" || cfg.OpenAI.ApiKey == "sk-YOUR_OPENAI_API_KEY" {
         fmt.Fprintln(os.Stderr, "ПРЕДУПРЕЖДЕНИЕ: OPENAI_API_KEY не установлен!")
         // return errors.New("OPENAI_API_KEY is required")
    }
    // TODO: Добавить другие проверки (например, URL должны быть валидными)
    return nil
}