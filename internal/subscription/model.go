// Модель подписки

package subscription

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// Status представляет статус подписки
type Status string

const (
	StatusActive    Status = "active"
	StatusCancelled Status = "cancelled"
	StatusExpired   Status = "expired"
)

// Features представляет особенности плана подписки
type Features map[string]interface{}

// Value реализует интерфейс driver.Valuer для конвертации в JSONB
func (f Features) Value() (driver.Value, error) {
	if f == nil {
		return nil, nil
	}
	return json.Marshal(f)
}

// Scan реализует интерфейс sql.Scanner для чтения из JSONB
func (f *Features) Scan(value interface{}) error {
	if value == nil {
		*f = nil
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("тип данных не поддерживается для Features.Scan")
	}

	return json.Unmarshal(data, f)
}

// Plan представляет план подписки
type Plan struct {
	ID               int       `db:"id"`
	Code             string    `db:"code"`
	Name             string    `db:"name"`
	Description      string    `db:"description"`
	PriceMonthly     int       `db:"price_monthly"` // в копейках
	PriceYearly      int       `db:"price_yearly"`  // в копейках
	DailyNeurons     int       `db:"daily_neurons"`
	MaxRequestLength int       `db:"max_request_length"`
	ContextMessages  int       `db:"context_messages"`
	Features         Features  `db:"features"`
	IsActive         bool      `db:"is_active"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
}

// GetMonthlyPriceRub возвращает цену за месяц в рублях
func (p *Plan) GetMonthlyPriceRub() float64 {
	return float64(p.PriceMonthly) / 100.0
}

// GetYearlyPriceRub возвращает цену за год в рублях
func (p *Plan) GetYearlyPriceRub() float64 {
	return float64(p.PriceYearly) / 100.0
}

// GetYearlySavingPercent возвращает процент экономии при годовой подписке
func (p *Plan) GetYearlySavingPercent() int {
	if p.PriceMonthly == 0 {
		return 0
	}
	monthlyPrice := float64(p.PriceMonthly * 12)
	yearlyPrice := float64(p.PriceYearly)

	saving := (monthlyPrice - yearlyPrice) / monthlyPrice * 100
	return int(saving)
}

// GetWelcomeBonus возвращает бонус нейронов при первой подписке
func (p *Plan) GetWelcomeBonus() int {
	if bonus, ok := p.Features["welcome_bonus"].(float64); ok {
		return int(bonus)
	}
	return 0
}

// GetNeuronDiscount возвращает процент экономии нейронов при использовании
func (p *Plan) GetNeuronDiscount() int {
	if discount, ok := p.Features["neuron_discount"].(float64); ok {
		return int(discount)
	}
	return 0
}

// GetNeuronExpiryDays возвращает срок действия нейронов в днях
func (p *Plan) GetNeuronExpiryDays() int {
	if days, ok := p.Features["neuron_expiry_days"].(float64); ok {
		return int(days)
	}
	return 3 // По умолчанию 3 дня
}

// GetAvailableModels возвращает список доступных моделей
func (p *Plan) GetAvailableModels() []string {
	if models, ok := p.Features["available_models"].([]interface{}); ok {
		result := make([]string, len(models))
		for i, model := range models {
			if str, ok := model.(string); ok {
				result[i] = str
			}
		}
		return result
	}
	return []string{}
}

// HasPriorityProcessing возвращает true, если план имеет приоритетную обработку
func (p *Plan) HasPriorityProcessing() bool {
	if priority, ok := p.Features["priority_processing"].(bool); ok {
		return priority
	}
	return false
}

// Subscription представляет подписку пользователя
type Subscription struct {
	ID            int64     `db:"id"`
	UserID        int64     `db:"user_id"`
	PlanID        int       `db:"plan_id"`
	Status        Status    `db:"status"`
	StartDate     time.Time `db:"start_date"`
	EndDate       time.Time `db:"end_date"`
	AutoRenew     bool      `db:"auto_renew"`
	PaymentID     string    `db:"payment_id"`
	PaymentMethod string    `db:"payment_method"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`

	// Дополнительные поля (не из БД)
	Plan *Plan `db:"-"`
}

// IsActive возвращает true, если подписка активна
func (s *Subscription) IsActive() bool {
	return s.Status == StatusActive && time.Now().Before(s.EndDate)
}

// DaysLeft возвращает количество дней до окончания подписки
func (s *Subscription) DaysLeft() int {
	if !s.IsActive() {
		return 0
	}

	duration := s.EndDate.Sub(time.Now())
	return int(duration.Hours() / 24)
}

// IsFree возвращает true, если это бесплатная подписка
func (s *Subscription) IsFree() bool {
	if s.Plan == nil {
		return false
	}
	return s.Plan.Code == "free"
}

// HistoryEvent представляет запись в истории подписок
type HistoryEvent struct {
	ID             int64       `db:"id"`
	UserID         int64       `db:"user_id"`
	SubscriptionID int64       `db:"subscription_id"`
	EventType      string      `db:"event_type"`
	Details        interface{} `db:"details"`
	CreatedAt      time.Time   `db:"created_at"`
}

// Constants for history event types
const (
	EventCreated   = "created"
	EventRenewed   = "renewed"
	EventCancelled = "cancelled"
	EventExpired   = "expired"
	EventChanged   = "changed"
)
