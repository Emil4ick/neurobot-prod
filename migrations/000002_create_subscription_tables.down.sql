-- migrations/000002_create_subscription_tables.down.sql
DROP TABLE IF EXISTS subscription_history;
DROP TABLE IF EXISTS user_subscriptions;
DROP TABLE IF EXISTS subscription_plans;