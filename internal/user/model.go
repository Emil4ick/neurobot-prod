package user

import (
	"database/sql"
	"time"
)

// User представляет информацию о пользователе
type User struct {
	ID           int64          `db:"id" json:"id"`
	TelegramID   int64          `db:"telegram_id" json:"telegram_id"`
	Username     sql.NullString `db:"username" json:"username,omitempty"`
	FirstName    string         `db:"first_name" json:"first_name"`
	LastName     sql.NullString `db:"last_name" json:"last_name,omitempty"`
	LanguageCode sql.NullString `db:"language_code" json:"language_code,omitempty"`
	IsBot        bool           `db:"is_bot" json:"is_bot"`
	CreatedAt    time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time      `db:"updated_at" json:"updated_at"`
}

// ToUserDTO преобразует User в UserDTO
func (u *User) ToUserDTO() UserDTO {
	return UserDTO{
		ID:           u.ID,
		TelegramID:   u.TelegramID,
		Username:     u.Username.String,
		FirstName:    u.FirstName,
		LastName:     u.LastName.String,
		LanguageCode: u.LanguageCode.String,
		IsBot:        u.IsBot,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

// UserDTO представляет упрощенную информацию о пользователе для передачи без sql.Null* типов
type UserDTO struct {
	ID           int64     `json:"id"`
	TelegramID   int64     `json:"telegram_id"`
	Username     string    `json:"username,omitempty"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name,omitempty"`
	LanguageCode string    `json:"language_code,omitempty"`
	IsBot        bool      `json:"is_bot"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
