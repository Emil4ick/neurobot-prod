-- migrations/000002_create_subscription_tables.up.sql
-- Таблицы для системы подписок

-- Типы подписок
CREATE TABLE IF NOT EXISTS subscription_plans (
    id SERIAL PRIMARY KEY,
    code VARCHAR(20) NOT NULL UNIQUE,   -- Код плана (free, premium, pro)
    name VARCHAR(100) NOT NULL,         -- Название плана
    description TEXT,                   -- Описание плана
    price_monthly INTEGER NOT NULL,     -- Цена за месяц (в копейках)
    price_yearly INTEGER NOT NULL,      -- Цена за год (в копейках)
    daily_neurons INTEGER NOT NULL,     -- Количество нейронов в день
    max_request_length INTEGER NOT NULL,-- Максимальная длина запроса
    context_messages INTEGER NOT NULL,  -- Количество сообщений в контексте
    features JSONB NOT NULL,            -- Особенности плана (JSON)
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Подписки пользователей
CREATE TABLE IF NOT EXISTS user_subscriptions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan_id INTEGER NOT NULL REFERENCES subscription_plans(id),
    status VARCHAR(20) NOT NULL,        -- active, cancelled, expired
    start_date TIMESTAMPTZ NOT NULL,
    end_date TIMESTAMPTZ NOT NULL,
    auto_renew BOOLEAN NOT NULL DEFAULT TRUE,
    payment_id VARCHAR(100),            -- ID платежа
    payment_method VARCHAR(50),         -- Метод оплаты
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- История подписок
CREATE TABLE IF NOT EXISTS subscription_history (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subscription_id BIGINT NOT NULL REFERENCES user_subscriptions(id),
    event_type VARCHAR(20) NOT NULL,    -- created, renewed, cancelled, expired
    details JSONB,                      -- Детали события (JSON)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Индексы
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_user_id ON user_subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_status ON user_subscriptions(status);
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_end_date ON user_subscriptions(end_date);
CREATE INDEX IF NOT EXISTS idx_subscription_history_user_id ON subscription_history(user_id);

-- Начальные данные для планов подписок
INSERT INTO subscription_plans (code, name, description, price_monthly, price_yearly, daily_neurons, max_request_length, context_messages, features, is_active)
VALUES 
('free', 'Бесплатный', 'Базовый доступ к нейросетям', 0, 0, 5, 500, 0, 
 '{"available_models": ["gpt-3.5-turbo", "claude-3-haiku", "gemini-1.0-pro", "grok-1"], "neuron_expiry_days": 3}', true),

('premium', 'Премиум', 'Расширенный доступ к нейросетям с дополнительными функциями', 39900, 399000, 30, 2000, 10, 
 '{"available_models": ["gpt-3.5-turbo", "claude-3-haiku", "gemini-1.0-pro", "grok-1", "gpt-4o-mini", "claude-3-sonnet", "gemini-1.5-pro"], "welcome_bonus": 100, "neuron_discount": 10, "neuron_expiry_days": 7}', true),

('pro', 'Профессиональный', 'Полный доступ ко всем нейросетям и функциям', 79900, 799000, 100, 0, 30, 
 '{"available_models": ["gpt-3.5-turbo", "claude-3-haiku", "gemini-1.0-pro", "grok-1", "gpt-4o-mini", "claude-3-sonnet", "gemini-1.5-pro", "gpt-4o", "claude-3-opus", "grok-2"], "welcome_bonus": 300, "neuron_discount": 20, "priority_processing": true, "neuron_expiry_days": 30}', true);