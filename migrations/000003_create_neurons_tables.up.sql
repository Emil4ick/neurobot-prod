-- migrations/000003_create_neurons_tables.up.sql
-- Таблицы для системы внутренней валюты "Нейроны"

-- Баланс пользователей
CREATE TABLE IF NOT EXISTS user_neuron_balance (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    balance INTEGER NOT NULL DEFAULT 0,          -- Текущий баланс нейронов
    lifetime_earned INTEGER NOT NULL DEFAULT 0,  -- Всего получено за все время
    lifetime_spent INTEGER NOT NULL DEFAULT 0,   -- Всего потрачено за все время
    last_daily_reward_at TIMESTAMPTZ,            -- Время последнего ежедневного начисления
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_user_balance UNIQUE (user_id)
);

-- Транзакции с нейронами
CREATE TABLE IF NOT EXISTS neuron_transactions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount INTEGER NOT NULL,                     -- Сумма транзакции (положительная или отрицательная)
    balance_after INTEGER NOT NULL,              -- Баланс после транзакции
    transaction_type VARCHAR(20) NOT NULL,       -- daily, purchase, usage, referral, bonus, subscription, admin
    description TEXT NOT NULL,                   -- Описание транзакции
    expires_at TIMESTAMPTZ,                      -- Время истечения срока действия нейронов (для начислений)
    reference_id VARCHAR(100),                   -- ID связанной сущности (платеж, запрос и т.д.)
    metadata JSONB,                              -- Дополнительные данные (JSON)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Пакеты нейронов для покупки
CREATE TABLE IF NOT EXISTS neuron_packages (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,                  -- Название пакета
    amount INTEGER NOT NULL,                     -- Количество нейронов
    bonus_amount INTEGER NOT NULL DEFAULT 0,     -- Бонусное количество
    price INTEGER NOT NULL,                      -- Цена в копейках
    sort_order INTEGER NOT NULL DEFAULT 0,       -- Порядок сортировки
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Использование нейросетей
CREATE TABLE IF NOT EXISTS llm_usage (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    model_name VARCHAR(50) NOT NULL,             -- Название модели
    prompt_tokens INTEGER NOT NULL,              -- Количество токенов запроса
    completion_tokens INTEGER NOT NULL,          -- Количество токенов ответа
    neurons_cost INTEGER NOT NULL,               -- Стоимость в нейронах
    transaction_id BIGINT REFERENCES neuron_transactions(id), -- Связанная транзакция
    request_hash VARCHAR(64),                    -- Хэш запроса (для кэширования)
    request_text TEXT,                           -- Текст запроса
    response_text TEXT,                          -- Текст ответа
    metadata JSONB,                              -- Дополнительные данные (JSON)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Индексы
CREATE INDEX IF NOT EXISTS idx_user_neuron_balance_user_id ON user_neuron_balance(user_id);
CREATE INDEX IF NOT EXISTS idx_neuron_transactions_user_id ON neuron_transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_neuron_transactions_type ON neuron_transactions(transaction_type);
CREATE INDEX IF NOT EXISTS idx_neuron_transactions_created_at ON neuron_transactions(created_at);
CREATE INDEX IF NOT EXISTS idx_llm_usage_user_id ON llm_usage(user_id);
CREATE INDEX IF NOT EXISTS idx_llm_usage_model_name ON llm_usage(model_name);
CREATE INDEX IF NOT EXISTS idx_llm_usage_request_hash ON llm_usage(request_hash);
CREATE INDEX IF NOT EXISTS idx_llm_usage_created_at ON llm_usage(created_at);

-- Начальные данные для пакетов нейронов
INSERT INTO neuron_packages (name, amount, bonus_amount, price, sort_order, is_active)
VALUES 
('50 Нейронов', 50, 0, 9900, 1, true),
('150 Нейронов', 150, 20, 24900, 2, true),
('350 Нейронов', 350, 50, 49900, 3, true),
('700 Нейронов', 700, 140, 89900, 4, true),
('1200 Нейронов', 1200, 300, 149900, 5, true);