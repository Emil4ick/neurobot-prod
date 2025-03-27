-- migrations/000001_create_users_table.down.sql
-- Откат миграции: удаление таблицы users

DROP INDEX IF EXISTS idx_users_telegram_id;
DROP TABLE IF EXISTS users;