-- migrations/000004_create_loyalty_tables.up.sql
-- Таблицы для системы лояльности

-- Уровни пользователей
CREATE TABLE IF NOT EXISTS user_levels (
    id SERIAL PRIMARY KEY,
    level INTEGER NOT NULL UNIQUE,               -- Номер уровня
    name VARCHAR(100) NOT NULL,                  -- Название уровня
    min_xp INTEGER NOT NULL,                     -- Минимальное количество опыта
    neuron_bonus_percent INTEGER NOT NULL DEFAULT 0, -- Бонус к ежедневным нейронам (%)
    features JSONB,                              -- Дополнительные особенности уровня
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Опыт пользователей
CREATE TABLE IF NOT EXISTS user_experience (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    current_xp INTEGER NOT NULL DEFAULT 0,       -- Текущий опыт
    current_level INTEGER NOT NULL DEFAULT 1,    -- Текущий уровень
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_user_experience UNIQUE (user_id)
);

-- История получения опыта
CREATE TABLE IF NOT EXISTS xp_history (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount INTEGER NOT NULL,                     -- Количество опыта
    source VARCHAR(50) NOT NULL,                 -- Источник (message, daily, referral, purchase)
    description TEXT NOT NULL,                   -- Описание
    reference_id VARCHAR(100),                   -- ID связанной сущности
    metadata JSONB,                              -- Дополнительные данные (JSON)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Достижения
CREATE TABLE IF NOT EXISTS achievements (
    id SERIAL PRIMARY KEY,
    code VARCHAR(50) NOT NULL UNIQUE,            -- Уникальный код достижения
    name VARCHAR(100) NOT NULL,                  -- Название достижения
    description TEXT NOT NULL,                   -- Описание достижения
    icon_url VARCHAR(255),                       -- URL иконки
    xp_reward INTEGER NOT NULL DEFAULT 0,        -- Награда в опыте
    neuron_reward INTEGER NOT NULL DEFAULT 0,    -- Награда в нейронах
    difficulty VARCHAR(20) NOT NULL,             -- easy, medium, hard
    is_hidden BOOLEAN NOT NULL DEFAULT FALSE,    -- Скрытое достижение
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Достижения пользователей
CREATE TABLE IF NOT EXISTS user_achievements (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    achievement_id INTEGER NOT NULL REFERENCES achievements(id),
    unlocked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reward_claimed BOOLEAN NOT NULL DEFAULT FALSE, -- Получена ли награда
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_user_achievement UNIQUE (user_id, achievement_id)
);

-- Промокоды
CREATE TABLE IF NOT EXISTS promocodes (
    id SERIAL PRIMARY KEY,
    code VARCHAR(50) NOT NULL UNIQUE,            -- Уникальный код
    discount_type VARCHAR(20) NOT NULL,          -- percent, fixed, neurons, free_trial
    discount_value INTEGER NOT NULL,             -- Размер скидки или количество нейронов
    max_uses INTEGER,                            -- Максимальное количество использований (NULL - без ограничений)
    current_uses INTEGER NOT NULL DEFAULT 0,     -- Текущее количество использований
    subscription_plan_id INTEGER REFERENCES subscription_plans(id), -- Для какого плана (NULL - для всех)
    start_date TIMESTAMPTZ,                      -- Дата начала действия (NULL - бессрочно)
    end_date TIMESTAMPTZ,                        -- Дата окончания действия (NULL - бессрочно)
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_by_admin VARCHAR(100),               -- Кто создал промокод
    metadata JSONB,                              -- Дополнительные данные (JSON)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Использование промокодов
CREATE TABLE IF NOT EXISTS promocode_usages (
    id BIGSERIAL PRIMARY KEY,
    promocode_id INTEGER NOT NULL REFERENCES promocodes(id),
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    discount_amount INTEGER NOT NULL,            -- Размер скидки или количество нейронов
    reference_id VARCHAR(100),                   -- ID связанной сущности (подписка, транзакция)
    metadata JSONB,                              -- Дополнительные данные (JSON)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Индексы
CREATE INDEX IF NOT EXISTS idx_user_experience_user_id ON user_experience(user_id);
CREATE INDEX IF NOT EXISTS idx_xp_history_user_id ON xp_history(user_id);
CREATE INDEX IF NOT EXISTS idx_user_achievements_user_id ON user_achievements(user_id);
CREATE INDEX IF NOT EXISTS idx_promocode_usages_user_id ON promocode_usages(user_id);
CREATE INDEX IF NOT EXISTS idx_promocode_usages_promocode_id ON promocode_usages(promocode_id);

-- Начальные данные для уровней пользователей
INSERT INTO user_levels (level, name, min_xp, neuron_bonus_percent, features)
VALUES 
(1, 'Начинающий', 0, 0, '{"icon": "novice.png"}'),
(5, 'Продвинутый', 500, 5, '{"icon": "advanced.png"}'),
(10, 'Опытный', 2000, 10, '{"icon": "experienced.png"}'),
(20, 'Эксперт', 5000, 15, '{"icon": "expert.png"}'),
(30, 'Профессионал', 10000, 20, '{"icon": "professional.png"}'),
(50, 'Мастер', 20000, 25, '{"icon": "master.png"}');

-- Начальные данные для достижений
INSERT INTO achievements (code, name, description, xp_reward, neuron_reward, difficulty, is_hidden)
VALUES 
('first_message', 'Первый шаг', 'Отправьте свой первый запрос нейросети', 10, 0, 'easy', false),
('daily_streak_7', 'Недельный марафон', 'Используйте бота 7 дней подряд', 50, 5, 'medium', false),
('daily_streak_30', 'Месячный марафон', 'Используйте бота 30 дней подряд', 200, 20, 'hard', false),
('try_all_models', 'Исследователь', 'Используйте все доступные модели нейросетей', 100, 10, 'medium', false),
('invite_friend', 'Проводник', 'Пригласите друга, который активно использует бота', 50, 5, 'easy', false),
('premium_subscription', 'Поддержка проекта', 'Оформите премиум подписку', 100, 0, 'medium', false),
('pro_subscription', 'VIP-персона', 'Оформите профессиональную подписку', 200, 0, 'hard', false),
('hidden_command', 'Секретное знание', 'Найдите секретную команду бота', 50, 5, 'medium', true);