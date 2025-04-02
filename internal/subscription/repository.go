// Репозиторий Нейронов

package subscription

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Repository представляет репозиторий для работы с подписками
type Repository struct {
	db *sql.DB
}

// NewRepository создает новый репозиторий подписок
func NewRepository(db *sql.DB) *Repository {
	return &Repository{
		db: db,
	}
}

// GetAllPlans возвращает все доступные планы подписок
func (r *Repository) GetAllPlans(ctx context.Context) ([]*Plan, error) {
	query := `
		SELECT id, code, name, description, price_monthly, price_yearly, 
			   daily_neurons, max_request_length, context_messages, features, 
			   is_active, created_at, updated_at
		FROM subscription_plans
		WHERE is_active = true
		ORDER BY price_monthly ASC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения планов подписок: %w", err)
	}
	defer rows.Close()

	var plans []*Plan
	for rows.Next() {
		plan := &Plan{}
		err := rows.Scan(
			&plan.ID,
			&plan.Code,
			&plan.Name,
			&plan.Description,
			&plan.PriceMonthly,
			&plan.PriceYearly,
			&plan.DailyNeurons,
			&plan.MaxRequestLength,
			&plan.ContextMessages,
			&plan.Features,
			&plan.IsActive,
			&plan.CreatedAt,
			&plan.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования плана подписки: %w", err)
		}
		plans = append(plans, plan)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при итерации планов подписок: %w", err)
	}

	return plans, nil
}

// GetPlanByID находит план подписки по его ID
func (r *Repository) GetPlanByID(ctx context.Context, planID int) (*Plan, error) {
	query := `
		SELECT id, code, name, description, price_monthly, price_yearly, 
			   daily_neurons, max_request_length, context_messages, features, 
			   is_active, created_at, updated_at
		FROM subscription_plans
		WHERE id = $1
	`

	plan := &Plan{}
	err := r.db.QueryRowContext(ctx, query, planID).Scan(
		&plan.ID,
		&plan.Code,
		&plan.Name,
		&plan.Description,
		&plan.PriceMonthly,
		&plan.PriceYearly,
		&plan.DailyNeurons,
		&plan.MaxRequestLength,
		&plan.ContextMessages,
		&plan.Features,
		&plan.IsActive,
		&plan.CreatedAt,
		&plan.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // План не найден
		}
		return nil, fmt.Errorf("ошибка получения плана подписки: %w", err)
	}

	return plan, nil
}

// GetPlanByCode находит план подписки по его коду
func (r *Repository) GetPlanByCode(ctx context.Context, code string) (*Plan, error) {
	query := `
		SELECT id, code, name, description, price_monthly, price_yearly, 
			   daily_neurons, max_request_length, context_messages, features, 
			   is_active, created_at, updated_at
		FROM subscription_plans
		WHERE code = $1 AND is_active = true
	`

	plan := &Plan{}
	err := r.db.QueryRowContext(ctx, query, code).Scan(
		&plan.ID,
		&plan.Code,
		&plan.Name,
		&plan.Description,
		&plan.PriceMonthly,
		&plan.PriceYearly,
		&plan.DailyNeurons,
		&plan.MaxRequestLength,
		&plan.ContextMessages,
		&plan.Features,
		&plan.IsActive,
		&plan.CreatedAt,
		&plan.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // План не найден
		}
		return nil, fmt.Errorf("ошибка получения плана подписки: %w", err)
	}

	return plan, nil
}

// GetActiveSubscription получает активную подписку пользователя
func (r *Repository) GetActiveSubscription(ctx context.Context, userID int64) (*Subscription, error) {
	query := `
		SELECT s.id, s.user_id, s.plan_id, s.status, s.start_date, s.end_date, 
			   s.auto_renew, s.payment_id, s.payment_method, s.created_at, s.updated_at
		FROM user_subscriptions s
		WHERE s.user_id = $1 AND s.status = $2 AND s.end_date > NOW()
		ORDER BY s.end_date DESC
		LIMIT 1
	`

	sub := &Subscription{}
	err := r.db.QueryRowContext(ctx, query, userID, StatusActive).Scan(
		&sub.ID,
		&sub.UserID,
		&sub.PlanID,
		&sub.Status,
		&sub.StartDate,
		&sub.EndDate,
		&sub.AutoRenew,
		&sub.PaymentID,
		&sub.PaymentMethod,
		&sub.CreatedAt,
		&sub.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// Если активная подписка не найдена, проверим наличие бесплатного плана
			freePlan, err := r.GetPlanByCode(ctx, "free")
			if err != nil {
				return nil, err
			}

			if freePlan != nil {
				// Создаем бесплатную подписку
				now := time.Now()
				sub := &Subscription{
					UserID:    userID,
					PlanID:    freePlan.ID,
					Status:    StatusActive,
					StartDate: now,
					EndDate:   now.AddDate(100, 0, 0), // Бесплатная подписка на 100 лет вперед
					AutoRenew: true,
					Plan:      freePlan,
				}
				return sub, nil
			}

			return nil, nil // Активная подписка не найдена
		}
		return nil, fmt.Errorf("ошибка получения активной подписки: %w", err)
	}

	// Получаем данные о плане подписки
	plan, err := r.GetPlanByID(ctx, sub.PlanID)
	if err != nil {
		return nil, err
	}
	sub.Plan = plan

	return sub, nil
}

// CreateSubscription создает новую подписку
func (r *Repository) CreateSubscription(ctx context.Context, sub *Subscription) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer tx.Rollback()

	// Сначала отменяем текущие активные подписки
	_, err = tx.ExecContext(ctx, `
		UPDATE user_subscriptions
		SET status = $1, updated_at = NOW()
		WHERE user_id = $2 AND status = $3 AND end_date > NOW()
	`, StatusCancelled, sub.UserID, StatusActive)
	if err != nil {
		return fmt.Errorf("ошибка отмены текущих подписок: %w", err)
	}

	// Создаем новую подписку
	query := `
		INSERT INTO user_subscriptions (
			user_id, plan_id, status, start_date, end_date, 
			auto_renew, payment_id, payment_method
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at
	`

	err = tx.QueryRowContext(
		ctx,
		query,
		sub.UserID,
		sub.PlanID,
		sub.Status,
		sub.StartDate,
		sub.EndDate,
		sub.AutoRenew,
		sub.PaymentID,
		sub.PaymentMethod,
	).Scan(&sub.ID, &sub.CreatedAt, &sub.UpdatedAt)

	if err != nil {
		return fmt.Errorf("ошибка создания подписки: %w", err)
	}

	// Записываем в историю
	details := map[string]interface{}{
		"plan_id":    sub.PlanID,
		"start_date": sub.StartDate,
		"end_date":   sub.EndDate,
	}

	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("ошибка при сериализации деталей истории: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO subscription_history (user_id, subscription_id, event_type, details)
		VALUES ($1, $2, $3, $4)
	`, sub.UserID, sub.ID, EventCreated, detailsJSON)

	if err != nil {
		return fmt.Errorf("ошибка записи в историю подписок: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ошибка подтверждения транзакции: %w", err)
	}

	return nil
}

// CancelSubscription отменяет подписку пользователя
func (r *Repository) CancelSubscription(ctx context.Context, subscriptionID int64, userID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer tx.Rollback()

	// Отменяем подписку
	result, err := tx.ExecContext(ctx, `
		UPDATE user_subscriptions
		SET status = $1, auto_renew = false, updated_at = NOW()
		WHERE id = $2 AND user_id = $3 AND status = $4
	`, StatusCancelled, subscriptionID, userID, StatusActive)
	if err != nil {
		return fmt.Errorf("ошибка отмены подписки: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("ошибка получения количества затронутых строк: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("подписка не найдена или уже отменена")
	}

	// Записываем в историю
	details := map[string]interface{}{
		"cancelled_at": time.Now(),
	}

	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("ошибка при сериализации деталей истории: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO subscription_history (user_id, subscription_id, event_type, details)
		VALUES ($1, $2, $3, $4)
	`, userID, subscriptionID, EventCancelled, detailsJSON)

	if err != nil {
		return fmt.Errorf("ошибка записи в историю подписок: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ошибка подтверждения транзакции: %w", err)
	}

	return nil
}

// GetSubscriptionHistory получает историю подписок пользователя
func (r *Repository) GetSubscriptionHistory(ctx context.Context, userID int64, limit, offset int) ([]*Subscription, error) {
	query := `
		SELECT s.id, s.user_id, s.plan_id, s.status, s.start_date, s.end_date, 
		       s.auto_renew, s.payment_id, s.payment_method, s.created_at, s.updated_at
		FROM user_subscriptions s
		WHERE s.user_id = $1
		ORDER BY s.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения истории подписок: %w", err)
	}
	defer rows.Close()

	var subs []*Subscription
	planIDs := make(map[int]bool)

	for rows.Next() {
		sub := &Subscription{}
		err := rows.Scan(
			&sub.ID,
			&sub.UserID,
			&sub.PlanID,
			&sub.Status,
			&sub.StartDate,
			&sub.EndDate,
			&sub.AutoRenew,
			&sub.PaymentID,
			&sub.PaymentMethod,
			&sub.CreatedAt,
			&sub.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования подписки: %w", err)
		}

		subs = append(subs, sub)
		planIDs[sub.PlanID] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при итерации подписок: %w", err)
	}

	// Загружаем планы для всех подписок
	plans := make(map[int]*Plan)
	for planID := range planIDs {
		plan, err := r.GetPlanByID(ctx, planID)
		if err != nil {
			return nil, err
		}
		if plan != nil {
			plans[planID] = plan
		}
	}

	// Устанавливаем план для каждой подписки
	for _, sub := range subs {
		if plan, ok := plans[sub.PlanID]; ok {
			sub.Plan = plan
		}
	}

	return subs, nil
}

// GetExpiringSubscriptions получает подписки, истекающие в указанный период
func (r *Repository) GetExpiringSubscriptions(ctx context.Context, daysThreshold int) ([]*Subscription, error) {
	query := `
		SELECT s.id, s.user_id, s.plan_id, s.status, s.start_date, s.end_date, 
		       s.auto_renew, s.payment_id, s.payment_method, s.created_at, s.updated_at
		FROM user_subscriptions s
		WHERE s.status = $1 
		  AND s.end_date > NOW() 
		  AND s.end_date < NOW() + INTERVAL '1 day' * $2
		  AND s.auto_renew = true
		ORDER BY s.end_date ASC
	`

	rows, err := r.db.QueryContext(ctx, query, StatusActive, daysThreshold)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения истекающих подписок: %w", err)
	}
	defer rows.Close()

	var subs []*Subscription
	planIDs := make(map[int]bool)

	for rows.Next() {
		sub := &Subscription{}
		err := rows.Scan(
			&sub.ID,
			&sub.UserID,
			&sub.PlanID,
			&sub.Status,
			&sub.StartDate,
			&sub.EndDate,
			&sub.AutoRenew,
			&sub.PaymentID,
			&sub.PaymentMethod,
			&sub.CreatedAt,
			&sub.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования подписки: %w", err)
		}

		subs = append(subs, sub)
		planIDs[sub.PlanID] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при итерации подписок: %w", err)
	}

	// Загружаем планы для всех подписок
	plans := make(map[int]*Plan)
	for planID := range planIDs {
		plan, err := r.GetPlanByID(ctx, planID)
		if err != nil {
			return nil, err
		}
		if plan != nil {
			plans[planID] = plan
		}
	}

	// Устанавливаем план для каждой подписки
	for _, sub := range subs {
		if plan, ok := plans[sub.PlanID]; ok {
			sub.Plan = plan
		}
	}

	return subs, nil
}

// ExpireSubscriptions помечает просроченные подписки как истекшие
func (r *Repository) ExpireSubscriptions(ctx context.Context) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer tx.Rollback()

	// Получаем просроченные подписки
	rows, err := tx.QueryContext(ctx, `
		SELECT id, user_id
		FROM user_subscriptions
		WHERE status = $1 AND end_date < NOW()
		FOR UPDATE
	`, StatusActive)

	if err != nil {
		return 0, fmt.Errorf("ошибка получения просроченных подписок: %w", err)
	}

	type expiredSub struct {
		ID     int64
		UserID int64
	}

	var expiredSubs []expiredSub
	for rows.Next() {
		var sub expiredSub
		if err := rows.Scan(&sub.ID, &sub.UserID); err != nil {
			rows.Close()
			return 0, fmt.Errorf("ошибка сканирования просроченной подписки: %w", err)
		}
		expiredSubs = append(expiredSubs, sub)
	}
	rows.Close()

	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("ошибка при итерации просроченных подписок: %w", err)
	}

	// Обновляем статус подписок
	for _, sub := range expiredSubs {
		_, err := tx.ExecContext(ctx, `
			UPDATE user_subscriptions
			SET status = $1, updated_at = NOW()
			WHERE id = $2
		`, StatusExpired, sub.ID)

		if err != nil {
			return 0, fmt.Errorf("ошибка обновления статуса подписки: %w", err)
		}

		// Записываем в историю
		details := map[string]interface{}{
			"expired_at": time.Now(),
		}

		detailsJSON, err := json.Marshal(details)
		if err != nil {
			return 0, fmt.Errorf("ошибка при сериализации деталей истории: %w", err)
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO subscription_history (user_id, subscription_id, event_type, details)
			VALUES ($1, $2, $3, $4)
		`, sub.UserID, sub.ID, EventExpired, detailsJSON)

		if err != nil {
			return 0, fmt.Errorf("ошибка записи в историю подписок: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("ошибка подтверждения транзакции: %w", err)
	}

	return len(expiredSubs), nil
}
