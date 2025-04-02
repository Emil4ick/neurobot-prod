-- migrations/000005_create_referral_tables.up.sql
-- Таблицы для реферальной системы

-- Реферальные коды пользователей
CREATE TABLE IF NOT EXISTS user_referral_codes (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    referral_code VARCHAR(20) NOT NULL UNIQUE,   -- Уникальный реферальный код
    total_referrals INTEGER NOT NULL DEFAULT 0,  -- Общее количество приглашенных
    active_referrals INTEGER NOT NULL DEFAULT 0, -- Активных приглашенных (использующих бота)
    total_earnings INTEGER NOT NULL DEFAULT 0,   -- Всего заработано нейронов через рефералов
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_user_referral_code UNIQUE (user_id)
);

-- Реферальные отношения
CREATE TABLE IF NOT EXISTS user_referrals (
    id BIGSERIAL PRIMARY KEY,
    referrer_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE, -- Кто пригласил
    referred_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE, -- Кого пригласил
    referral_code VARCHAR(20) NOT NULL,          -- Использованный код
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, active, inactive
    welcome_reward_claimed BOOLEAN NOT NULL DEFAULT FALSE, -- Получена ли стартовая награда
    welcome_reward_amount INTEGER,               -- Размер стартовой награды
    activated_at TIMESTAMPTZ,                    -- Когда реферал стал активным
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_referred UNIQUE (referred_id)
);

-- История реферальных вознаграждений
CREATE TABLE IF NOT EXISTS referral_rewards (
    id BIGSERIAL PRIMARY KEY,
    referrer_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE, -- Кто получил награду
    referred_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE, -- За кого получена награда
    amount INTEGER NOT NULL,                     -- Размер награды в нейронах
    reward_type VARCHAR(20) NOT NULL,            -- welcome, subscription, purchase, usage
    transaction_id BIGINT REFERENCES neuron_transactions(id), -- Связанная транзакция
    reference_id VARCHAR(100),                   -- ID связанной сущности
    metadata JSONB,                              -- Дополнительные данные (JSON)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Индексы
CREATE INDEX IF NOT EXISTS idx_user_referral_codes_user_id ON user_referral_codes(user_id);
CREATE INDEX IF NOT EXISTS idx_user_referrals_referrer_id ON user_referrals(referrer_id);
CREATE INDEX IF NOT EXISTS idx_user_referrals_referred_id ON user_referrals(referred_id);
CREATE INDEX IF NOT EXISTS idx_referral_rewards_referrer_id ON referral_rewards(referrer_id);
CREATE INDEX IF NOT EXISTS idx_referral_rewards_referred_id ON referral_rewards(referred_id);