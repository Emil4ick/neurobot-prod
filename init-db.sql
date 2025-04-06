-- Файл для первичной инициализации базы данных
-- Создаем таблицу пользователей
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    telegram_id BIGINT UNIQUE NOT NULL,
    username VARCHAR(32),
    first_name VARCHAR(64) NOT NULL,
    last_name VARCHAR(64),
    language_code VARCHAR(10),
    is_bot BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Индекс для быстрого поиска по telegram_id
CREATE INDEX IF NOT EXISTS idx_users_telegram_id ON users(telegram_id);

-- Таблица планов подписок
CREATE TABLE IF NOT EXISTS subscription_plans (
    id SERIAL PRIMARY KEY,
    code VARCHAR(20) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    price_monthly INTEGER NOT NULL,
    price_yearly INTEGER NOT NULL,
    daily_neurons INTEGER NOT NULL,
    max_request_length INTEGER NOT NULL,
    context_messages INTEGER NOT NULL,
    features JSONB NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Таблица подписок пользователей
CREATE TABLE IF NOT EXISTS user_subscriptions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan_id INTEGER NOT NULL REFERENCES subscription_plans(id),
    status VARCHAR(20) NOT NULL,
    start_date TIMESTAMPTZ NOT NULL,
    end_date TIMESTAMPTZ NOT NULL,
    auto_renew BOOLEAN NOT NULL DEFAULT TRUE,
    payment_id VARCHAR(100),
    payment_method VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Таблица истории подписок
CREATE TABLE IF NOT EXISTS subscription_history (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subscription_id BIGINT NOT NULL REFERENCES user_subscriptions(id),
    event_type VARCHAR(20) NOT NULL,
    details JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Таблица баланса нейронов
CREATE TABLE IF NOT EXISTS user_neuron_balance (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    balance INTEGER NOT NULL DEFAULT 0,
    lifetime_earned INTEGER NOT NULL DEFAULT 0,
    lifetime_spent INTEGER NOT NULL DEFAULT 0,
    last_daily_reward_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_user_balance UNIQUE (user_id)
);

-- Таблица транзакций с нейронами
CREATE TABLE IF NOT EXISTS neuron_transactions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount INTEGER NOT NULL,
    balance_after INTEGER NOT NULL,
    transaction_type VARCHAR(20) NOT NULL,
    description TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    reference_id VARCHAR(100),
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Таблица пакетов нейронов
CREATE TABLE IF NOT EXISTS neuron_packages (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    amount INTEGER NOT NULL,
    bonus_amount INTEGER NOT NULL DEFAULT 0,
    price INTEGER NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Таблица использования нейросетей
CREATE TABLE IF NOT EXISTS llm_usage (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    model_name VARCHAR(50) NOT NULL,
    prompt_tokens INTEGER NOT NULL,
    completion_tokens INTEGER NOT NULL,
    neurons_cost INTEGER NOT NULL,
    transaction_id BIGINT REFERENCES neuron_transactions(id),
    request_hash VARCHAR(64),
    request_text TEXT,
    response_text TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Начальные данные для планов подписок
INSERT INTO subscription_plans (code, name, description, price_monthly, price_yearly, daily_neurons, max_request_length, context_messages, features, is_active)
VALUES 
('free', 'Бесплатный', 'Базовый доступ к нейросетям', 0, 0, 5, 500, 0, 
 '{"available_models": ["gpt-3.5-turbo", "claude-3-haiku", "gemini-1.0-pro", "grok-1"], "neuron_expiry_days": 3}', true),

('premium', 'Премиум', 'Расширенный доступ к нейросетям с дополнительными функциями', 39900, 399000, 30, 2000, 10, 
 '{"available_models": ["gpt-3.5-turbo", "claude-3-haiku", "gemini-1.0-pro", "grok-1", "gpt-4o-mini", "claude-3-sonnet", "gemini-1.5-pro"], "welcome_bonus": 100, "neuron_discount": 10, "neuron_expiry_days": 7}', true),

('pro', 'Профессиональный', 'Полный доступ ко всем нейросетям и функциям', 79900, 799000, 100, 0, 30, 
 '{"available_models": ["gpt-3.5-turbo", "claude-3-haiku", "gemini-1.0-pro", "grok-1", "gpt-4o-mini", "claude-3-sonnet", "gemini-1.5-pro", "gpt-4o", "claude-3-opus", "grok-2"], "welcome_bonus": 300, "neuron_discount": 20, "priority_processing": true, "neuron_expiry_days": 30}', true);

-- Начальные данные для пакетов нейронов
INSERT INTO neuron_packages (name, amount, bonus_amount, price, sort_order, is_active)
VALUES 
('50 Нейронов', 50, 0, 9900, 1, true),
('150 Нейронов', 150, 20, 24900, 2, true),
('350 Нейронов', 350, 50, 49900, 3, true),
('700 Нейронов', 700, 140, 89900, 4, true),
('1200 Нейронов', 1200, 300, 149900, 5, true);