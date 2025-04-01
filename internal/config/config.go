package config

import (
    "errors"
	"fmt"
	"os"
	"strings"
	"time"
	"github.com/spf13/viper"
)
// Структуры БЕЗ СЕКРЕТОВ
type AppConfig struct { Env string `mapstructure:"env"`; LogLevel string `mapstructure:"log_level"` }
type NATSConfig struct { /*...*/ URL string `mapstructure:"url"`; ReconnectWaitSeconds int `mapstructure:"reconnect_wait_seconds"`; MaxReconnects int `mapstructure:"max_reconnects"`; TimeoutSeconds int `mapstructure:"timeout_seconds"`; Subjects NATSSubjects `mapstructure:"subjects"` }
type NATSSubjects struct { TelegramUpdates string `mapstructure:"telegram_updates"`; LLMTasks string `mapstructure:"llm_tasks"`; LLMResults string `mapstructure:"llm_results"` }
type TelegramConfig struct { WebhookPath string `mapstructure:"webhook_path"`; Token string; SecretToken string }
type DBConfig struct { /*...*/ Host string `mapstructure:"host"`; Port int `mapstructure:"port"`; User string `mapstructure:"user"`; Name string `mapstructure:"name"`; SSLMode string `mapstructure:"sslmode"`; PoolMinConns int32 `mapstructure:"pool_min_conns"`; PoolMaxConns int32 `mapstructure:"pool_max_conns"`; Password string }
type RedisConfig struct { /*...*/ Addr string `mapstructure:"addr"`; DB int `mapstructure:"db"`; DefaultTTLSeconds int `mapstructure:"default_ttl_seconds"`; Password string }
type OpenAIConfig struct { Model string `mapstructure:"model"`; ApiKey string }
type ServiceConfig struct { Webhook WebhookServiceConfig `mapstructure:"webhook"` }
type WebhookServiceConfig struct { Port int `mapstructure:"port"`; Metrics MetricsConfig `mapstructure:"metrics"` }
type MetricsConfig struct { Enabled bool `mapstructure:"enabled"`; Path string `mapstructure:"path"`; Port int `mapstructure:"port"` }
type Config struct { App AppConfig; NATS NATSConfig; Telegram TelegramConfig; DB DBConfig; Redis RedisConfig; OpenAI OpenAIConfig; Services ServiceConfig }
// Функции для time.Duration
func (c NATSConfig) GetReconnectWait() time.Duration { return time.Duration(c.ReconnectWaitSeconds) * time.Second }
func (c NATSConfig) GetTimeout() time.Duration { return time.Duration(c.TimeoutSeconds) * time.Second }
func (c RedisConfig) GetDefaultTTL() time.Duration { return time.Duration(c.DefaultTTLSeconds) * time.Second }
// Функция LoadConfig с чтением из ENV
func LoadConfig(configPath string) (*Config, error) {
    v := viper.New(); setDefaults(v); v.AddConfigPath(configPath); v.SetConfigName("config"); v.SetConfigType("yaml")
    v.SetEnvKeyReplacer(strings.NewReplacer(".", "_")); v.AutomaticEnv()
    if err := v.ReadInConfig(); err != nil { if _, ok := err.(viper.ConfigFileNotFoundError); !ok { return nil, fmt.Errorf("ошибка чтения файла конфигурации: %w", err) } }
    var cfg Config; if err := v.Unmarshal(&cfg); err != nil { return nil, fmt.Errorf("не удалось разобрать конфигурацию: %w", err) }
    // Читаем секреты из ENV
    cfg.Telegram.Token = v.GetString("telegram.token"); cfg.Telegram.SecretToken = v.GetString("telegram.secret_token")
    cfg.DB.Password = v.GetString("db.password"); cfg.Redis.Password = v.GetString("redis.password"); cfg.OpenAI.ApiKey = v.GetString("openai.api_key")
    if err := validateSecrets(cfg); err != nil { fmt.Fprintf(os.Stderr, "ПРЕДУПРЕЖДЕНИЕ: %v\n", err) /* return nil, err */ }
    return &cfg, nil
}
// validateSecrets
func validateSecrets(cfg Config) error {
    if cfg.Telegram.Token == "" { return errors.New("секрет TELEGRAM_TOKEN не установлен") }
    if cfg.Telegram.SecretToken == "" { return errors.New("секрет TELEGRAM_SECRET_TOKEN не установлен") }
    if cfg.DB.Password == "" { return errors.New("секрет DB_PASSWORD не установлен") }
    if cfg.OpenAI.ApiKey == "" { return errors.New("секрет OPENAI_API_KEY не установлен") }
    return nil
}
// setDefaults БЕЗ СЕКРЕТОВ
func setDefaults(v *viper.Viper) {
     v.SetDefault("app.env", "development"); v.SetDefault("app.log_level", "info")
     v.SetDefault("nats.url", "nats://localhost:4222"); v.SetDefault("nats.reconnect_wait_seconds", 2); v.SetDefault("nats.max_reconnects", 60); v.SetDefault("nats.timeout_seconds", 1)
     v.SetDefault("nats.subjects.telegram_updates", "tg.updates.v1"); v.SetDefault("nats.subjects.llm_tasks", "llm.tasks.v1"); v.SetDefault("nats.subjects.llm_results", "llm.results.v1")
     v.SetDefault("telegram.webhook_path", "/webhook")
     v.SetDefault("db.host", "localhost"); v.SetDefault("db.port", 5432); v.SetDefault("db.user", "neuro_user"); v.SetDefault("db.name", "neurobot_db"); v.SetDefault("db.sslmode", "disable"); v.SetDefault("db.pool_min_conns", 2); v.SetDefault("db.pool_max_conns", 10)
     v.SetDefault("redis.addr", "localhost:6379"); v.SetDefault("redis.db", 0); v.SetDefault("redis.default_ttl_seconds", 600)
     v.SetDefault("openai.model", "gpt-3.5-turbo")
     v.SetDefault("services.webhook.port", 8080); v.SetDefault("services.webhook.metrics.enabled", false); v.SetDefault("services.webhook.metrics.path", "/metrics"); v.SetDefault("services.webhook.metrics.port", 9091)
}
// ConnectionString
func (c *DBConfig) ConnectionString() string { return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s&pool_min_conns=%d&pool_max_conns=%d", c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode, c.PoolMinConns, c.PoolMaxConns) }
