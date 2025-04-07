package postgres

import (
	"context"
	"database/sql"
	"fmt"
)

// Уровни изоляции транзакций
const (
	IsolationLevelReadCommitted  = "READ COMMITTED"
	IsolationLevelRepeatableRead = "REPEATABLE READ"
	IsolationLevelSerializable   = "SERIALIZABLE"
)

// TransactionManager обеспечивает работу с транзакциями
type TransactionManager struct {
	db *sql.DB
}

// NewTransactionManager создает новый менеджер транзакций
func NewTransactionManager(db *sql.DB) *TransactionManager {
	return &TransactionManager{
		db: db,
	}
}

// WithTransaction выполняет функцию fn в контексте транзакции
func (tm *TransactionManager) WithTransaction(ctx context.Context, isolationLevel string, fn func(*sql.Tx) error) error {
	// Начинаем транзакцию с указанным уровнем изоляции
	tx, err := tm.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}

	// Устанавливаем уровень изоляции, если он указан
	if isolationLevel != "" {
		_, err = tx.ExecContext(ctx, fmt.Sprintf("SET TRANSACTION ISOLATION LEVEL %s", isolationLevel))
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("ошибка установки уровня изоляции: %w", err)
		}
	}

	// Выполняем переданную функцию в контексте транзакции
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	// Фиксируем транзакцию
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ошибка фиксации транзакции: %w", err)
	}

	return nil
}
