-- migrations/000004_create_loyalty_tables.down.sql
DROP TABLE IF EXISTS promocode_usages;
DROP TABLE IF EXISTS promocodes;
DROP TABLE IF EXISTS user_achievements;
DROP TABLE IF EXISTS achievements;
DROP TABLE IF EXISTS xp_history;
DROP TABLE IF EXISTS user_experience;
DROP TABLE IF EXISTS user_levels;