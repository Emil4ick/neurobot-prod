package config

import (
	"errors"
	"fmt"
	"os"
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

// LLMConfig содержит настройки для различных моделей ИИ
type LLMConfig struct {
	OpenAI OpenAIConfig `mapstructure:"openai"`
	Claude ClaudeConfig `mapstructure:"claude"`
	Grok   GrokConfig   `mapstructure:"grok"`
	Gemini GeminiConfig `mapstructure:"gemini"`
}

// OpenAIConfig содержит настройки для OpenAI API
type OpenAIConfig struct {
	BaseModel         string `mapstructure:"base_model"`
	PremiumModel      string `mapstructure:"premium_model"`
	ProModel          string `mapstructure:"pro_model"`
	BaseTokenLimit    int    `mapstructure:"base_token_limit"`
	PremiumTokenLimit int    `mapstructure:"premium_token_limit"`
	ProTokenLimit     int    `mapstructure:"pro_token_limit"`
	ApiKey            string // Заполняется из ENV
}

// ClaudeConfig содержит настройки для Claude API
type ClaudeConfig struct {
	BaseModel         string `mapstructure:"base_model"`
	PremiumModel      string `mapstructure:"premium_model"`
	ProModel          string `mapstructure:"pro_model"`
	BaseTokenLimit    int    `mapstructure:"base_token_limit"`
	PremiumTokenLimit int    `mapstructure:"premium_token_limit"`
	ProTokenLimit     int    `mapstructure:"pro_token_limit"`
	ApiKey            string // Заполняется из ENV
}

// GrokConfig содержит настройки для Grok API
type GrokConfig struct {
	BaseModel      string `mapstructure:"base_model"`
	ProModel       string `mapstructure:"pro_model"`
	BaseTokenLimit int    `mapstructure:"base_token_limit"`
	ProTokenLimit  int    `mapstructure:"pro_token_limit"`
	ApiKey         string // Заполняется из ENV
}

// GeminiConfig содержит настройки для Gemini API
type GeminiConfig struct {
	BaseModel         string `mapstructure:"base_model"`
	PremiumModel      string `mapstructure:"premium_model"`
	BaseTokenLimit    int    `mapstructure:"base_token_limit"`
	PremiumTokenLimit int    `mapstructure:"premium_token_limit"`
	ApiKey            string // Заполняется из ENV
}

// ServiceConfig содержит настройки для различных сервисов
type ServiceConfig struct {
	Webhook WebhookServiceConfig `mapstructure:"webhook"`
	API     APIServiceConfig     `mapstructure:"api"`
}

// WebhookServiceConfig содержит настройки для Webhook сервиса
type WebhookServiceConfig struct {
	Port    int           `mapstructure:"port"`
	Metrics MetricsConfig `mapstructure:"metrics"`
}

// APIServiceConfig содержит настройки для API сервиса
type APIServiceConfig struct {
	Port               int      `mapstructure:"port"`
	CORSAllowedOrigins []string `mapstructure:"cors_allowed_origins"`
	JWTSecret          string   // Заполняется из ENV
	JWTExpiryHours     int      `mapstructure:"jwt_expiry_hours"`
}

// MetricsConfig содержит настройки для сбора метрик
type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
	Port    int    `mapstructure:"port"`
}

// SubscriptionConfig содержит настройки для системы подписок
type SubscriptionConfig struct {
	FreeNeuronsPerDay    int `mapstructure:"free_neurons_per_day"`
	PremiumNeuronsPerDay int `mapstructure:"premium_neurons_per_day"`
	ProNeuronsPerDay     int `mapstructure:"pro_neurons_per_day"`

	FreeMaxRequestLength    int `mapstructure:"free_max_request_length"`
	PremiumMaxRequestLength int `mapstructure:"premium_max_request_length"`

	// Стоимость в нейронах для разных моделей
	BaseModelCost    int `mapstructure:"base_model_cost"`
	PremiumModelCost int `mapstructure:"premium_model_cost"`
	ProModelCost     int `mapstructure:"pro_model_cost"`
}

// PaymentConfig содержит настройки для системы платежей
type PaymentConfig struct {
	YooKassa YooKassaConfig `mapstructure:"yookassa"`
	// Можно добавить другие платежные системы
}

// YooKassaConfig содержит настройки для ЮKassa
type YooKassaConfig struct {
	ShopID      string `mapstructure:"shop_id"`
	CallbackURL string `mapstructure:"callback_url"`
	SecretKey   string // Заполняется из ENV
}

// Config содержит полную конфигурацию приложения
type Config struct {
	App          AppConfig
	NATS         NATSConfig
	Telegram     TelegramConfig
	DB           DBConfig
	Redis        RedisConfig
	LLM          LLMConfig
	Services     ServiceConfig
	Subscription SubscriptionConfig
	Payment      PaymentConfig
}

// Функции для time.Duration
func (c NATSConfig) GetReconnectWait() time.Duration {
	return time.Duration(c.ReconnectWaitSeconds) * time.Second
}

func (c NATSConfig) GetTimeout() time.Duration {
	return time.Duration(c.TimeoutSeconds) * time.Second
}

func (c RedisConfig) GetDefaultTTL() time.Duration {
	return time.Duration(c.DefaultTTLSeconds) * time.Second
}

func (c RedisConfig) GetCacheTTL() time.Duration {
	return time.Duration(c.CacheTTLSeconds) * time.Second
}

// LoadConfig загружает конфигурацию из файла и переменных окружения
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()
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

	// Читаем секреты из ENV
	cfg.Telegram.Token = v.GetString("telegram.token")
	cfg.Telegram.SecretToken = v.GetString("telegram.secret_token")
	cfg.DB.Password = v.GetString("db.password")
	cfg.Redis.Password = v.GetString("redis.password")
	cfg.LLM.OpenAI.ApiKey = v.GetString("llm.openai.api_key")
	cfg.LLM.Claude.ApiKey = v.GetString("llm.claude.api_key")
	cfg.LLM.Grok.ApiKey = v.GetString("llm.grok.api_key")
	cfg.LLM.Gemini.ApiKey = v.GetString("llm.gemini.api_key")
	cfg.Services.API.JWTSecret = v.GetString("services.api.jwt_secret")
	cfg.Payment.YooKassa.SecretKey = v.GetString("payment.yookassa.secret_key")

	if err := validateSecrets(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "ПРЕДУПРЕЖДЕНИЕ: %v\n", err)
	}

	return &cfg, nil
}

// validateSecrets проверяет наличие необходимых секретов
func validateSecrets(cfg Config) error {
	missingSecrets := []string{}

	if cfg.Telegram.Token == "" {
		missingSecrets = append(missingSecrets, "TELEGRAM_TOKEN")
	}
	if cfg.Telegram.SecretToken == "" {
		missingSecrets = append(missingSecrets, "TELEGRAM_SECRET_TOKEN")
	}
	if cfg.DB.Password == "" {
		missingSecrets = append(missingSecrets, "DB_PASSWORD")
	}
	if cfg.LLM.OpenAI.ApiKey == "" {
		missingSecrets = append(missingSecrets, "LLM_OPENAI_API_KEY")
	}

	if len(missingSecrets) > 0 {
		return errors.New("следующие секреты не установлены: " + strings.Join(missingSecrets, ", "))
	}

	return nil
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

	// LLM - OpenAI
	v.SetDefault("llm.openai.base_model", "gpt-3.5-turbo")
	v.SetDefault("llm.openai.premium_model", "gpt-4o-mini")
	v.SetDefault("llm.openai.pro_model", "gpt-4o")
	v.SetDefault("llm.openai.base_token_limit", 4000)
	v.SetDefault("llm.openai.premium_token_limit", 8000)
	v.SetDefault("llm.openai.pro_token_limit", 16000)

	// LLM - Claude
	v.SetDefault("llm.claude.base_model", "claude-3-haiku-20240307")
	v.SetDefault("llm.claude.premium_model", "claude-3-sonnet-20240229")
	v.SetDefault("llm.claude.pro_model", "claude-3-opus-20240229")
	v.SetDefault("llm.claude.base_token_limit", 4000)
	v.SetDefault("llm.claude.premium_token_limit", 8000)
	v.SetDefault("llm.claude.pro_token_limit", 16000)

	// LLM - Grok
	v.SetDefault("llm.grok.base_model", "grok-1")
	v.SetDefault("llm.grok.pro_model", "grok-2")
	v.SetDefault("llm.grok.base_token_limit", 4000)
	v.SetDefault("llm.grok.pro_token_limit", 8000)

	// LLM - Gemini
	v.SetDefault("llm.gemini.base_model", "gemini-1.0-pro")
	v.SetDefault("llm.gemini.premium_model", "gemini-1.5-pro")
	v.SetDefault("llm.gemini.base_token_limit", 4000)
	v.SetDefault("llm.gemini.premium_token_limit", 8000)

	// Services - Webhook
	v.SetDefault("services.webhook.port", 8080)
	v.SetDefault("services.webhook.metrics.enabled", false)
	v.SetDefault("services.webhook.metrics.path", "/metrics")
	v.SetDefault("services.webhook.metrics.port", 9091)

	// Services - API
	v.SetDefault("services.api.port", 8081)
	v.SetDefault("services.api.cors_allowed_origins", []string{"https://yourneuro.ru", "https://t.me"})
	v.SetDefault("services.api.jwt_expiry_hours", 24)

	// Subscription
	v.SetDefault("subscription.free_neurons_per_day", 5)
	v.SetDefault("subscription.premium_neurons_per_day", 30)
	v.SetDefault("subscription.pro_neurons_per_day", 100)
	v.SetDefault("subscription.free_max_request_length", 500)
	v.SetDefault("subscription.premium_max_request_length", 2000)
	v.SetDefault("subscription.base_model_cost", 1)
	v.SetDefault("subscription.premium_model_cost", 3)
	v.SetDefault("subscription.pro_model_cost", 5)

	// Payment - YooKassa
	v.SetDefault("payment.yookassa.callback_url", "https://yourneuro.ru/api/v1/payments/callback")
}

// ConnectionString генерирует строку подключения к PostgreSQL
func (c *DBConfig) ConnectionString() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s&pool_min_conns=%d&pool_max_conns=%d",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode, c.PoolMinConns, c.PoolMaxConns)
}
