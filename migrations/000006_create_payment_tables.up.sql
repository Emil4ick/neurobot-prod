-- migrations/000006_create_payment_tables.up.sql
-- Таблицы для системы платежей

-- Платежи
CREATE TABLE IF NOT EXISTS payments (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    external_id VARCHAR(100) UNIQUE,             -- Внешний ID платежа
    payment_type VARCHAR(20) NOT NULL,           -- subscription, neurons
    item_id INTEGER,                             -- ID связанного объекта (план подписки или пакет нейронов)
    amount INTEGER NOT NULL,                     -- Сумма в копейках
    status VARCHAR(20) NOT NULL,                 -- pending, succeeded, failed, canceled
    payment_method VARCHAR(50),                  -- Метод оплаты
    payment_provider VARCHAR(50) NOT NULL,       -- Провайдер платежа (yookassa, etc)
    invoice_id VARCHAR(100),                     -- ID счета в платежной системе
    receipt_id VARCHAR(100),                     -- ID чека в платежной системе
    payment_url VARCHAR(255),                    -- URL для оплаты
    metadata JSONB,                              -- Дополнительные данные (JSON)
    expires_at TIMESTAMPTZ,                      -- Время истечения срока действия платежа
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Уведомления от платежных систем
CREATE TABLE IF NOT EXISTS payment_notifications (
    id BIGSERIAL PRIMARY KEY,
    payment_id BIGINT REFERENCES payments(id),
    provider VARCHAR(50) NOT NULL,               -- Провайдер платежа
    external_id VARCHAR(100),                    -- Внешний ID уведомления
    notification_type VARCHAR(50) NOT NULL,      -- Тип уведомления
    payload JSONB NOT NULL,                      -- Содержимое уведомления
    is_processed BOOLEAN NOT NULL DEFAULT FALSE, -- Обработано ли уведомление
    error_message TEXT,                          -- Сообщение об ошибке (если есть)
    processed_at TIMESTAMPTZ,                    -- Когда было обработано
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Индексы
CREATE INDEX IF NOT EXISTS idx_payments_user_id ON payments(user_id);
CREATE INDEX IF NOT EXISTS idx_payments_external_id ON payments(external_id);
CREATE INDEX IF NOT EXISTS idx_payments_status ON payments(status);
CREATE INDEX IF NOT EXISTS idx_payments_created_at ON payments(created_at);
CREATE INDEX IF NOT EXISTS idx_payment_notifications_payment_id ON payment_notifications(payment_id);
CREATE INDEX IF NOT EXISTS idx_payment_notifications_is_processed ON payment_notifications(is_processed);
CREATE INDEX IF NOT EXISTS idx_payment_notifications_created_at ON payment_notifications(created_at);