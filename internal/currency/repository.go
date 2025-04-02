// Репозиторий Нейронов

package currency

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Repository представляет репозиторий для работы с нейронами
type Repository struct {
	db *sql.DB
}

// NewRepository создает новый репозиторий нейронов
func NewRepository(db *sql.DB) *Repository {
	return &Repository{
		db: db,
	}
}

// GetBalance получает баланс нейронов пользователя
func (r *Repository) GetBalance(ctx context.Context, userID int64) (*Balance, error) {
	query := `
		SELECT id, user_id, balance, lifetime_earned, lifetime_spent, last_daily_reward_at, created_at, updated_at
		FROM user_neuron_balance
		WHERE user_id = $1
	`

	balance := &Balance{}
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&balance.ID,
		&balance.UserID,
		&balance.Balance,
		&balance.LifetimeEarned,
		&balance.LifetimeSpent,
		&balance.LastDailyRewardAt,
		&balance.CreatedAt,
		&balance.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// Если записи нет, создаем новую с нулевым балансом
			return r.createInitialBalance(ctx, userID)
		}
		return nil, fmt.Errorf("ошибка получения баланса: %w", err)
	}

	return balance, nil
}

// createInitialBalance создает начальный баланс для нового пользователя
func (r *Repository) createInitialBalance(ctx context.Context, userID int64) (*Balance, error) {
	query := `
		INSERT INTO user_neuron_balance (user_id, balance, lifetime_earned, lifetime_spent)
		VALUES ($1, 0, 0, 0)
		RETURNING id, user_id, balance, lifetime_earned, lifetime_spent, last_daily_reward_at, created_at, updated_at
	`

	balance := &Balance{
		UserID: userID,
	}

	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&balance.ID,
		&balance.UserID,
		&balance.Balance,
		&balance.LifetimeEarned,
		&balance.LifetimeSpent,
		&balance.LastDailyRewardAt,
		&balance.CreatedAt,
		&balance.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("ошибка создания начального баланса: %w", err)
	}

	return balance, nil
}

// AddTransaction добавляет новую транзакцию и обновляет баланс
func (r *Repository) AddTransaction(ctx context.Context, tx *Transaction) error {
	// Начинаем транзакцию в БД
	dbTx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer dbTx.Rollback()

	// Получаем текущий баланс пользователя с блокировкой строки
	var currentBalance, lifetimeEarned, lifetimeSpent int
	var lastDailyRewardAt *time.Time

	err = dbTx.QueryRowContext(ctx, `
		SELECT balance, lifetime_earned, lifetime_spent, last_daily_reward_at
		FROM user_neuron_balance 
		WHERE user_id = $1 
		FOR UPDATE
	`, tx.UserID).Scan(&currentBalance, &lifetimeEarned, &lifetimeSpent, &lastDailyRewardAt)

	if err != nil {
		if err == sql.ErrNoRows {
			// Создаем запись баланса, если ее нет
			_, err = dbTx.ExecContext(ctx, `
				INSERT INTO user_neuron_balance (user_id, balance, lifetime_earned, lifetime_spent)
				VALUES ($1, 0, 0, 0)
			`, tx.UserID)
			if err != nil {
				return fmt.Errorf("ошибка создания баланса: %w", err)
			}
			currentBalance = 0
			lifetimeEarned = 0
			lifetimeSpent = 0
		} else {
			return fmt.Errorf("ошибка получения баланса: %w", err)
		}
	}

	// Рассчитываем новый баланс
	newBalance := currentBalance + tx.Amount
	if newBalance < 0 {
		return errors.New("недостаточно нейронов на балансе")
	}

	// Обновляем lifetime метрики и последнюю дату ежедневного вознаграждения
	if tx.Amount > 0 {
		lifetimeEarned += tx.Amount
	} else {
		lifetimeSpent += -tx.Amount
	}

	updateFields := "balance = $1, lifetime_earned = $2, lifetime_spent = $3, updated_at = NOW()"
	updateArgs := []interface{}{newBalance, lifetimeEarned, lifetimeSpent, tx.UserID}

	// Обновляем дату последнего ежедневного вознаграждения, если это daily транзакция
	if tx.TransactionType == TypeDaily {
		now := time.Now()
		updateFields = "balance = $1, lifetime_earned = $2, lifetime_spent = $3, last_daily_reward_at = $4, updated_at = NOW()"
		updateArgs = []interface{}{newBalance, lifetimeEarned, lifetimeSpent, now, tx.UserID}
	}

	// Обновляем баланс
	_, err = dbTx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE user_neuron_balance
		SET %s
		WHERE user_id = $%d
	`, updateFields, len(updateArgs)), updateArgs...)

	if err != nil {
		return fmt.Errorf("ошибка обновления баланса: %w", err)
	}

	// Устанавливаем баланс после транзакции
	tx.BalanceAfter = newBalance

	// Вставляем запись транзакции
	query := `
		INSERT INTO neuron_transactions (
			user_id, amount, balance_after, transaction_type, 
			description, expires_at, reference_id, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at
	`

	err = dbTx.QueryRowContext(
		ctx,
		query,
		tx.UserID,
		tx.Amount,
		tx.BalanceAfter,
		tx.TransactionType,
		tx.Description,
		tx.ExpiresAt,
		tx.ReferenceID,
		tx.Metadata,
	).Scan(&tx.ID, &tx.CreatedAt)

	if err != nil {
		return fmt.Errorf("ошибка создания транзакции: %w", err)
	}

	// Подтверждаем транзакцию
	if err := dbTx.Commit(); err != nil {
		return fmt.Errorf("ошибка подтверждения транзакции: %w", err)
	}

	return nil
}

// GetTransactionHistory получает историю транзакций пользователя
func (r *Repository) GetTransactionHistory(ctx context.Context, userID int64, limit, offset int) ([]*Transaction, error) {
	query := `
		SELECT id, user_id, amount, balance_after, transaction_type, 
		       description, expires_at, reference_id, metadata, created_at
		FROM neuron_transactions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения истории транзакций: %w", err)
	}
	defer rows.Close()

	var transactions []*Transaction
	for rows.Next() {
		tx := &Transaction{}
		err := rows.Scan(
			&tx.ID,
			&tx.UserID,
			&tx.Amount,
			&tx.BalanceAfter,
			&tx.TransactionType,
			&tx.Description,
			&tx.ExpiresAt,
			&tx.ReferenceID,
			&tx.Metadata,
			&tx.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования транзакции: %w", err)
		}
		transactions = append(transactions, tx)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при итерации транзакций: %w", err)
	}

	return transactions, nil
}

// GetAllPackages получает все доступные пакеты нейронов
func (r *Repository) GetAllPackages(ctx context.Context) ([]*Package, error) {
	query := `
		SELECT id, name, amount, bonus_amount, price, sort_order, is_active, created_at, updated_at
		FROM neuron_packages
		WHERE is_active = true
		ORDER BY sort_order ASC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения пакетов нейронов: %w", err)
	}
	defer rows.Close()

	var packages []*Package
	for rows.Next() {
		pkg := &Package{}
		err := rows.Scan(
			&pkg.ID,
			&pkg.Name,
			&pkg.Amount,
			&pkg.BonusAmount,
			&pkg.Price,
			&pkg.SortOrder,
			&pkg.IsActive,
			&pkg.CreatedAt,
			&pkg.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования пакета нейронов: %w", err)
		}
		packages = append(packages, pkg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при итерации пакетов нейронов: %w", err)
	}

	return packages, nil
}

// GetPackageByID получает пакет нейронов по ID
func (r *Repository) GetPackageByID(ctx context.Context, packageID int) (*Package, error) {
	query := `
		SELECT id, name, amount, bonus_amount, price, sort_order, is_active, created_at, updated_at
		FROM neuron_packages
		WHERE id = $1
	`

	pkg := &Package{}
	err := r.db.QueryRowContext(ctx, query, packageID).Scan(
		&pkg.ID,
		&pkg.Name,
		&pkg.Amount,
		&pkg.BonusAmount,
		&pkg.Price,
		&pkg.SortOrder,
		&pkg.IsActive,
		&pkg.CreatedAt,
		&pkg.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Пакет не найден
		}
		return nil, fmt.Errorf("ошибка получения пакета нейронов: %w", err)
	}

	return pkg, nil
}

// AddLLMUsage добавляет запись об использовании нейросети
func (r *Repository) AddLLMUsage(ctx context.Context, usage *LLMUsage) error {
	query := `
		INSERT INTO llm_usage (
			user_id, model_name, prompt_tokens, completion_tokens, 
			neurons_cost, transaction_id, request_hash, 
			request_text, response_text, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		usage.UserID,
		usage.ModelName,
		usage.PromptTokens,
		usage.CompletionTokens,
		usage.NeuronsCost,
		usage.TransactionID,
		usage.RequestHash,
		usage.RequestText,
		usage.ResponseText,
		usage.Metadata,
	).Scan(&usage.ID, &usage.CreatedAt)

	if err != nil {
		return fmt.Errorf("ошибка добавления записи об использовании нейросети: %w", err)
	}

	return nil
}

// GetLLMUsageByHash получает запись об использовании нейросети по хэшу запроса
func (r *Repository) GetLLMUsageByHash(ctx context.Context, requestHash string) (*LLMUsage, error) {
	query := `
		SELECT id, user_id, model_name, prompt_tokens, completion_tokens, 
		       neurons_cost, transaction_id, request_hash, 
		       request_text, response_text, metadata, created_at
		FROM llm_usage
		WHERE request_hash = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	usage := &LLMUsage{}
	err := r.db.QueryRowContext(ctx, query, requestHash).Scan(
		&usage.ID,
		&usage.UserID,
		&usage.ModelName,
		&usage.PromptTokens,
		&usage.CompletionTokens,
		&usage.NeuronsCost,
		&usage.TransactionID,
		&usage.RequestHash,
		&usage.RequestText,
		&usage.ResponseText,
		&usage.Metadata,
		&usage.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Запись не найдена
		}
		return nil, fmt.Errorf("ошибка получения записи об использовании нейросети: %w", err)
	}

	return usage, nil
}

// GetUserLLMUsageStatistics получает статистику использования нейросетей пользователем
func (r *Repository) GetUserLLMUsageStatistics(ctx context.Context, userID int64) (map[string]int, error) {
	query := `
		SELECT model_name, COUNT(*) as count
		FROM llm_usage
		WHERE user_id = $1
		GROUP BY model_name
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения статистики использования нейросетей: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var modelName string
		var count int
		if err := rows.Scan(&modelName, &count); err != nil {
			return nil, fmt.Errorf("ошибка сканирования статистики: %w", err)
		}
		stats[modelName] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при итерации статистики: %w", err)
	}

	return stats, nil
}

// ExpireNeurons помечает истекшие нейроны как недоступные
func (r *Repository) ExpireNeurons(ctx context.Context) (int, error) {
	// Начинаем транзакцию в БД
	dbTx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer dbTx.Rollback()

	// Получаем истекшие транзакции, сгруппированные по пользователям
	query := `
		SELECT user_id, SUM(amount) as total_amount
		FROM neuron_transactions
		WHERE expires_at < NOW() AND amount > 0
			AND NOT EXISTS (
				SELECT 1 FROM neuron_transactions t2
				WHERE t2.reference_id = neuron_transactions.id::text
				AND t2.transaction_type = 'admin'
				AND t2.amount < 0
			)
		GROUP BY user_id
	`

	rows, err := dbTx.QueryContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("ошибка получения истекших нейронов: %w", err)
	}

	type expiredEntry struct {
		UserID int64
		Amount int
	}

	var expiredEntries []expiredEntry
	for rows.Next() {
		var entry expiredEntry
		if err := rows.Scan(&entry.UserID, &entry.Amount); err != nil {
			rows.Close()
			return 0, fmt.Errorf("ошибка сканирования истекших нейронов: %w", err)
		}
		expiredEntries = append(expiredEntries, entry)
	}
	rows.Close()

	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("ошибка при итерации истекших нейронов: %w", err)
	}

	// Создаем транзакции для списания истекших нейронов
	for _, entry := range expiredEntries {
		// Получаем текущий баланс пользователя
		var currentBalance int
		err := dbTx.QueryRowContext(ctx, `
			SELECT balance FROM user_neuron_balance 
			WHERE user_id = $1 
			FOR UPDATE
		`, entry.UserID).Scan(&currentBalance)

		if err != nil {
			return 0, fmt.Errorf("ошибка получения баланса для списания: %w", err)
		}

		// Если баланс меньше, чем количество истекших нейронов,
		// списываем только имеющийся баланс
		amountToExpire := entry.Amount
		if currentBalance < amountToExpire {
			amountToExpire = currentBalance
		}

		if amountToExpire <= 0 {
			continue
		}

		// Обновляем баланс
		newBalance := currentBalance - amountToExpire
		_, err = dbTx.ExecContext(ctx, `
			UPDATE user_neuron_balance
			SET balance = $1, updated_at = NOW()
			WHERE user_id = $2
		`, newBalance, entry.UserID)

		if err != nil {
			return 0, fmt.Errorf("ошибка обновления баланса при списании: %w", err)
		}

		// Создаем транзакцию списания
		_, err = dbTx.ExecContext(ctx, `
			INSERT INTO neuron_transactions (
				user_id, amount, balance_after, transaction_type, 
				description, reference_id
			)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, entry.UserID, -amountToExpire, newBalance, TypeAdmin, "Истечение срока действия нейронов", "expire_system")

		if err != nil {
			return 0, fmt.Errorf("ошибка создания транзакции списания: %w", err)
		}
	}

	// Подтверждаем транзакцию
	if err := dbTx.Commit(); err != nil {
		return 0, fmt.Errorf("ошибка подтверждения транзакции: %w", err)
	}

	return len(expiredEntries), nil
}
