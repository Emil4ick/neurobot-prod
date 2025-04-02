// Модель Нейронов

package currency

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// TransactionType представляет тип транзакции
type TransactionType string

const (
	TypeDaily        TransactionType = "daily"        // Ежедневное начисление
	TypePurchase     TransactionType = "purchase"     // Покупка Нейронов
	TypeUsage        TransactionType = "usage"        // Использование нейросети
	TypeReferral     TransactionType = "referral"     // Реферальная программа
	TypeBonus        TransactionType = "bonus"        // Бонусное начисление
	TypeSubscription TransactionType = "subscription" // Начисление за подписку
	TypeAdmin        TransactionType = "admin"        // Начисление администратором
	TypePromocode    TransactionType = "promocode"    // Начисление по промокоду
	TypeAchievement  TransactionType = "achievement"  // Начисление за достижение
)

// Metadata представляет дополнительные данные для транзакций
type Metadata map[string]interface{}

// Value реализует интерфейс driver.Valuer для конвертации в JSONB
func (m Metadata) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

// Scan реализует интерфейс sql.Scanner для чтения из JSONB
func (m *Metadata) Scan(value interface{}) error {
	if value == nil {
		*m = nil
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("тип данных не поддерживается для Metadata.Scan")
	}

	return json.Unmarshal(data, m)
}

// Balance представляет баланс нейронов пользователя
type Balance struct {
	ID                int64      `db:"id"`
	UserID            int64      `db:"user_id"`
	Balance           int        `db:"balance"`              // Текущий баланс
	LifetimeEarned    int        `db:"lifetime_earned"`      // Всего заработано
	LifetimeSpent     int        `db:"lifetime_spent"`       // Всего потрачено
	LastDailyRewardAt *time.Time `db:"last_daily_reward_at"` // Время последнего ежедневного вознаграждения
	CreatedAt         time.Time  `db:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at"`
}

// CanReceiveDailyReward проверяет, может ли пользователь получить ежедневное вознаграждение
func (b *Balance) CanReceiveDailyReward() bool {
	if b.LastDailyRewardAt == nil {
		return true
	}

	// Проверяем, прошло ли более 20 часов с момента последнего вознаграждения
	// (делаем запас в 4 часа, чтобы пользователь мог получать вознаграждение каждый день в одно и то же время)
	return time.Since(*b.LastDailyRewardAt) > 20*time.Hour
}

// Transaction представляет транзакцию с нейронами
type Transaction struct {
	ID              int64           `db:"id"`
	UserID          int64           `db:"user_id"`
	Amount          int             `db:"amount"`           // Может быть положительным или отрицательным
	BalanceAfter    int             `db:"balance_after"`    // Баланс после транзакции
	TransactionType TransactionType `db:"transaction_type"` // Тип транзакции
	Description     string          `db:"description"`      // Описание транзакции
	ExpiresAt       *time.Time      `db:"expires_at"`       // Время истечения срока действия нейронов
	ReferenceID     string          `db:"reference_id"`     // Связанный ID (например, ID платежа)
	Metadata        Metadata        `db:"metadata"`         // Дополнительные данные
	CreatedAt       time.Time       `db:"created_at"`
}

// Package представляет пакет нейронов для покупки
type Package struct {
	ID          int       `db:"id"`
	Name        string    `db:"name"`
	Amount      int       `db:"amount"`       // Количество нейронов
	BonusAmount int       `db:"bonus_amount"` // Бонусное количество нейронов
	Price       int       `db:"price"`        // Цена в копейках
	SortOrder   int       `db:"sort_order"`   // Порядок сортировки
	IsActive    bool      `db:"is_active"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// GetTotalAmount возвращает общее количество нейронов (основное + бонусное)
func (p *Package) GetTotalAmount() int {
	return p.Amount + p.BonusAmount
}

// GetPriceRub возвращает цену в рублях
func (p *Package) GetPriceRub() float64 {
	return float64(p.Price) / 100.0
}

// GetValueRatio возвращает соотношение нейронов к рублю (сколько нейронов за 1 рубль)
func (p *Package) GetValueRatio() float64 {
	if p.Price == 0 {
		return 0.0
	}
	return float64(p.GetTotalAmount()) / (float64(p.Price) / 100.0)
}

// LLMUsage представляет использование нейросети
type LLMUsage struct {
	ID               int64     `db:"id"`
	UserID           int64     `db:"user_id"`
	ModelName        string    `db:"model_name"`        // Название модели
	PromptTokens     int       `db:"prompt_tokens"`     // Количество токенов запроса
	CompletionTokens int       `db:"completion_tokens"` // Количество токенов ответа
	NeuronsCost      int       `db:"neurons_cost"`      // Стоимость в нейронах
	TransactionID    *int64    `db:"transaction_id"`    // Связанная транзакция
	RequestHash      string    `db:"request_hash"`      // Хэш запроса (для кэширования)
	RequestText      string    `db:"request_text"`      // Текст запроса
	ResponseText     string    `db:"response_text"`     // Текст ответа
	Metadata         Metadata  `db:"metadata"`          // Дополнительные данные
	CreatedAt        time.Time `db:"created_at"`
}
