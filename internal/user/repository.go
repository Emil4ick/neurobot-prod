package user

import (
	"context"
	"database/sql"
	"fmt"
)

// Repository представляет репозиторий для работы с пользователями
type Repository struct {
	db *sql.DB
}

// NewRepository создает новый репозиторий пользователей
func NewRepository(db *sql.DB) *Repository {
	return &Repository{
		db: db,
	}
}

// UpsertUser создает пользователя, если он не существует, или обновляет существующего
// Использует более эффективную стратегию "INSERT ON CONFLICT DO UPDATE"
func (r *Repository) UpsertUser(ctx context.Context, telegramID int64, username, firstName, lastName, languageCode string, isBot bool) (*User, error) {
	// Подготавливаем параметры, преобразуя пустые строки в NULL
	var usernameParam, lastNameParam, languageCodeParam sql.NullString

	if username != "" {
		usernameParam.String = username
		usernameParam.Valid = true
	}

	if lastName != "" {
		lastNameParam.String = lastName
		lastNameParam.Valid = true
	}

	if languageCode != "" {
		languageCodeParam.String = languageCode
		languageCodeParam.Valid = true
	}

	// Используем INSERT ... ON CONFLICT DO UPDATE для атомарной операции
	// Это устраняет race condition и позволяет в одном запросе создавать или обновлять данные
	query := `
		INSERT INTO users (telegram_id, username, first_name, last_name, language_code, is_bot, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (telegram_id) 
		DO UPDATE SET 
			username = COALESCE(EXCLUDED.username, users.username),
			first_name = EXCLUDED.first_name,
			last_name = COALESCE(EXCLUDED.last_name, users.last_name),
			language_code = COALESCE(EXCLUDED.language_code, users.language_code),
			is_bot = EXCLUDED.is_bot,
			updated_at = NOW()
		RETURNING id, telegram_id, username, first_name, last_name, language_code, is_bot, created_at, updated_at
	`

	// Выполняем запрос и получаем обновленные данные
	var user User
	err := r.db.QueryRowContext(
		ctx,
		query,
		telegramID,
		usernameParam,
		firstName,
		lastNameParam,
		languageCodeParam,
		isBot,
	).Scan(
		&user.ID,
		&user.TelegramID,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.LanguageCode,
		&user.IsBot,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("ошибка upsert пользователя: %w", err)
	}

	return &user, nil
}

// GetByTelegramID получает пользователя по его Telegram ID с использованием индекса
func (r *Repository) GetByTelegramID(ctx context.Context, telegramID int64) (*User, error) {
	query := `
		SELECT id, telegram_id, username, first_name, last_name, language_code, is_bot, created_at, updated_at
		FROM users
		WHERE telegram_id = $1
	`

	var user User
	err := r.db.QueryRowContext(ctx, query, telegramID).Scan(
		&user.ID,
		&user.TelegramID,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.LanguageCode,
		&user.IsBot,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Пользователь не найден
		}
		return nil, fmt.Errorf("ошибка получения пользователя: %w", err)
	}

	return &user, nil
}

// Дополнительные методы для работы с пользователями...

// GetUsers получает список пользователей с постраничной пагинацией
func (r *Repository) GetUsers(ctx context.Context, limit, offset int) ([]*User, error) {
	query := `
		SELECT id, telegram_id, username, first_name, last_name, language_code, is_bot, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения списка пользователей: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		err := rows.Scan(
			&user.ID,
			&user.TelegramID,
			&user.Username,
			&user.FirstName,
			&user.LastName,
			&user.LanguageCode,
			&user.IsBot,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования пользователя: %w", err)
		}
		users = append(users, &user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при итерации пользователей: %w", err)
	}

	return users, nil
}

// CountUsers возвращает общее количество пользователей в базе
func (r *Repository) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("ошибка подсчета пользователей: %w", err)
	}
	return count, nil
}
