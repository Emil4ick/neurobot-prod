-- migrations/000001_create_users_table.up.sql
-- Создаем таблицу для хранения пользователей

CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,                      -- Внутренний ID пользователя (автоинкремент)
    telegram_id BIGINT UNIQUE NOT NULL,         -- ID пользователя из Telegram (индекс создается автоматически для UNIQUE)
    username VARCHAR(32),                       -- Юзернейм (@username), может быть NULL. Макс. длина 32.
    first_name VARCHAR(64) NOT NULL,            -- Имя пользователя. Макс. длина 64.
    last_name VARCHAR(64),                      -- Фамилия, может быть NULL. Макс. длина 64.
    language_code VARCHAR(10),                  -- Языковой код (e.g., "en", "ru")
    is_bot BOOLEAN NOT NULL DEFAULT FALSE,      -- Флаг, что это бот
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), -- Время создания записи (с часовым поясом)
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()  -- Время последнего обновления записи
);

-- Индекс для быстрого поиска по telegram_id (хотя UNIQUE уже создает индекс, но явный не помешает)
CREATE INDEX IF NOT EXISTS idx_users_telegram_id ON users(telegram_id);

-- Комментарии к таблице и колонкам (хорошая практика)
COMMENT ON TABLE users IS 'Таблица для хранения информации о пользователях Telegram';
COMMENT ON COLUMN users.id IS 'Уникальный внутренний идентификатор пользователя';
COMMENT ON COLUMN users.telegram_id IS 'Уникальный идентификатор пользователя в Telegram';
COMMENT ON COLUMN users.username IS 'Юзернейм пользователя в Telegram (без @)';
COMMENT ON COLUMN users.first_name IS 'Имя пользователя в Telegram';
COMMENT ON COLUMN users.last_name IS 'Фамилия пользователя в Telegram';
COMMENT ON COLUMN users.language_code IS 'Языковой код пользователя из Telegram';
COMMENT ON COLUMN users.is_bot IS 'Является ли пользователь ботом';
COMMENT ON COLUMN users.created_at IS 'Время создания записи о пользователе';
COMMENT ON COLUMN users.updated_at IS 'Время последнего обновления записи о пользователе';