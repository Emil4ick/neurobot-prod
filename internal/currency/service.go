// Сервис для работы с Нейронами

package currency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"neurobot-prod/internal/subscription"
)

// Service предоставляет методы для работы с нейронами
type Service struct {
	repo       *Repository
	subService *subscription.Service
	log        *zap.Logger
}

// NewService создает новый сервис нейронов
func NewService(repo *Repository, subService *subscription.Service, log *zap.Logger) *Service {
	return &Service{
		repo:       repo,
		subService: subService,
		log:        log.Named("neurons_service"),
	}
}

// GetBalance получает текущий баланс пользователя
func (s *Service) GetBalance(ctx context.Context, userID int64) (*Balance, error) {
	balance, err := s.repo.GetBalance(ctx, userID)
	if err != nil {
		s.log.Error("Ошибка получения баланса",
			zap.Int64("user_id", userID),
			zap.Error(err))
		return nil, fmt.Errorf("ошибка получения баланса: %w", err)
	}
	return balance, nil
}

// AddDailyNeurons добавляет ежедневные нейроны пользователю
func (s *Service) AddDailyNeurons(ctx context.Context, userID int64, loyaltyBonusPercent int) (*Transaction, error) {
	// Получаем баланс пользователя
	balance, err := s.repo.GetBalance(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Проверяем, можно ли начислить ежедневное вознаграждение
	if !balance.CanReceiveDailyReward() {
		return nil, errors.New("ежедневное вознаграждение уже получено")
	}

	// Получаем подписку пользователя для определения количества нейронов
	dailyNeurons, err := s.subService.GetDailyNeurons(ctx, userID, loyaltyBonusPercent)
	if err != nil {
		return nil, err
	}

	// Получаем план подписки для определения срока действия нейронов
	plan, err := s.subService.GetSubscriptionPlan(ctx, userID)
	if err != nil {
		return nil, err
	}

	neuronExpiryDays := plan.GetNeuronExpiryDays()

	// Устанавливаем время истечения срока действия нейронов
	expiresAt := time.Now().AddDate(0, 0, neuronExpiryDays)

	// Создаем транзакцию
	tx := &Transaction{
		UserID:          userID,
		Amount:          dailyNeurons,
		TransactionType: TypeDaily,
		Description:     fmt.Sprintf("Ежедневное начисление: %d нейронов", dailyNeurons),
		ExpiresAt:       &expiresAt,
		Metadata: Metadata{
			"loyalty_bonus_percent": loyaltyBonusPercent,
			"expiry_days":           neuronExpiryDays,
			"subscription_plan":     plan.Code,
		},
	}

	err = s.repo.AddTransaction(ctx, tx)
	if err != nil {
		s.log.Error("Ошибка добавления ежедневных нейронов",
			zap.Int64("user_id", userID),
			zap.Int("amount", dailyNeurons),
			zap.Error(err))
		return nil, fmt.Errorf("ошибка добавления ежедневных нейронов: %w", err)
	}

	s.log.Info("Добавлены ежедневные нейроны",
		zap.Int64("user_id", userID),
		zap.Int("amount", dailyNeurons),
		zap.Int("balance", tx.BalanceAfter))

	return tx, nil
}

// AddNeurons добавляет нейроны на баланс пользователя
func (s *Service) AddNeurons(ctx context.Context, userID int64, amount int, txType TransactionType, description string, metadata Metadata, referenceID string, expiryDays int) (*Transaction, error) {
	if amount <= 0 {
		return nil, errors.New("сумма должна быть положительной")
	}

	// Устанавливаем время истечения срока действия нейронов
	var expiresAt *time.Time
	if expiryDays > 0 {
		expires := time.Now().AddDate(0, 0, expiryDays)
		expiresAt = &expires
	}

	// Создаем транзакцию
	tx := &Transaction{
		UserID:          userID,
		Amount:          amount,
		TransactionType: txType,
		Description:     description,
		ExpiresAt:       expiresAt,
		ReferenceID:     referenceID,
		Metadata:        metadata,
	}

	err := s.repo.AddTransaction(ctx, tx)
	if err != nil {
		s.log.Error("Ошибка добавления нейронов",
			zap.Int64("user_id", userID),
			zap.Int("amount", amount),
			zap.String("type", string(txType)),
			zap.Error(err))
		return nil, fmt.Errorf("ошибка добавления нейронов: %w", err)
	}

	s.log.Info("Нейроны успешно добавлены",
		zap.Int64("user_id", userID),
		zap.Int("amount", amount),
		zap.String("type", string(txType)),
		zap.Int("balance", tx.BalanceAfter))

	return tx, nil
}

// SpendNeurons списывает нейроны с баланса пользователя
func (s *Service) SpendNeurons(ctx context.Context, userID int64, amount int, txType TransactionType, description string, metadata Metadata, referenceID string) (*Transaction, error) {
	if amount <= 0 {
		return nil, errors.New("сумма должна быть положительной")
	}

	// Создаем транзакцию с отрицательной суммой для списания
	tx := &Transaction{
		UserID:          userID,
		Amount:          -amount, // Отрицательное значение для расхода
		TransactionType: txType,
		Description:     description,
		ReferenceID:     referenceID,
		Metadata:        metadata,
	}

	err := s.repo.AddTransaction(ctx, tx)
	if err != nil {
		s.log.Error("Ошибка списания нейронов",
			zap.Int64("user_id", userID),
			zap.Int("amount", amount),
			zap.String("type", string(txType)),
			zap.Error(err))
		return nil, fmt.Errorf("ошибка списания нейронов: %w", err)
	}

	s.log.Info("Нейроны успешно списаны",
		zap.Int64("user_id", userID),
		zap.Int("amount", amount),
		zap.String("type", string(txType)),
		zap.Int("balance", tx.BalanceAfter))

	return tx, nil
}

// GetTransactionHistory получает историю транзакций пользователя
func (s *Service) GetTransactionHistory(ctx context.Context, userID int64, limit, offset int) ([]*Transaction, error) {
	transactions, err := s.repo.GetTransactionHistory(ctx, userID, limit, offset)
	if err != nil {
		s.log.Error("Ошибка получения истории транзакций",
			zap.Int64("user_id", userID),
			zap.Error(err))
		return nil, fmt.Errorf("ошибка получения истории транзакций: %w", err)
	}
	return transactions, nil
}

// GetAvailablePackages получает список доступных пакетов нейронов
func (s *Service) GetAvailablePackages(ctx context.Context) ([]*Package, error) {
	packages, err := s.repo.GetAllPackages(ctx)
	if err != nil {
		s.log.Error("Ошибка получения пакетов нейронов", zap.Error(err))
		return nil, fmt.Errorf("ошибка получения пакетов нейронов: %w", err)
	}
	return packages, nil
}

// PurchaseNeuronsPackage обрабатывает покупку пакета нейронов
func (s *Service) PurchaseNeuronsPackage(ctx context.Context, userID int64, packageID int, paymentID string) (*Transaction, error) {
	// Получаем информацию о пакете
	pkg, err := s.repo.GetPackageByID(ctx, packageID)
	if err != nil {
		s.log.Error("Ошибка получения информации о пакете",
			zap.Int("package_id", packageID),
			zap.Error(err))
		return nil, fmt.Errorf("ошибка получения информации о пакете: %w", err)
	}

	if pkg == nil {
		return nil, errors.New("пакет не найден")
	}

	if !pkg.IsActive {
		return nil, errors.New("пакет недоступен для покупки")
	}

	// Получаем план подписки для определения срока действия нейронов
	plan, err := s.subService.GetSubscriptionPlan(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Устанавливаем время истечения срока действия нейронов
	neuronExpiryDays := plan.GetNeuronExpiryDays()

	// Создаем транзакцию для добавления нейронов
	totalAmount := pkg.GetTotalAmount()
	metadata := Metadata{
		"package_id":        packageID,
		"package_name":      pkg.Name,
		"base_amount":       pkg.Amount,
		"bonus_amount":      pkg.BonusAmount,
		"price_kopecks":     pkg.Price,
		"expiry_days":       neuronExpiryDays,
		"subscription_plan": plan.Code,
	}

	description := fmt.Sprintf("Покупка пакета '%s' (%d + %d бонус нейронов)",
		pkg.Name, pkg.Amount, pkg.BonusAmount)

	return s.AddNeurons(ctx, userID, totalAmount, TypePurchase, description, metadata, paymentID, neuronExpiryDays)
}

// HasEnoughNeurons проверяет, достаточно ли у пользователя нейронов
func (s *Service) HasEnoughNeurons(ctx context.Context, userID int64, amount int) (bool, error) {
	balance, err := s.repo.GetBalance(ctx, userID)
	if err != nil {
		return false, err
	}
	return balance.Balance >= amount, nil
}

// RecordLLMUsage записывает использование нейросети и списывает нейроны
func (s *Service) RecordLLMUsage(ctx context.Context, userID int64, modelName, requestText, responseText string, promptTokens, completionTokens, neuronsCost int, metadata Metadata) (*LLMUsage, error) {
	// Генерируем хэш запроса для кэширования
	requestHash := s.generateRequestHash(requestText, modelName)

	// Проверяем, был ли такой запрос ранее (поиск в кэше)
	cachedUsage, err := s.repo.GetLLMUsageByHash(ctx, requestHash)
	if err != nil {
		s.log.Error("Ошибка поиска запроса в кэше",
			zap.Int64("user_id", userID),
			zap.String("model", modelName),
			zap.Error(err))
	}

	// Если запрос найден в кэше, используем его
	if cachedUsage != nil {
		s.log.Info("Найден закэшированный ответ",
			zap.Int64("user_id", userID),
			zap.String("model", modelName),
			zap.String("hash", requestHash))

		// Создаем новую запись использования, но без списания нейронов
		usage := &LLMUsage{
			UserID:           userID,
			ModelName:        modelName,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			NeuronsCost:      0, // Не списываем нейроны за кэшированный ответ
			RequestHash:      requestHash,
			RequestText:      requestText,
			ResponseText:     cachedUsage.ResponseText, // Используем закэшированный ответ
			Metadata:         metadata,
		}

		// Добавляем информацию о кэшировании
		if usage.Metadata == nil {
			usage.Metadata = Metadata{}
		}
		usage.Metadata["cached"] = true
		usage.Metadata["cached_id"] = cachedUsage.ID

		// Записываем использование без списания нейронов
		err = s.repo.AddLLMUsage(ctx, usage)
		if err != nil {
			s.log.Error("Ошибка записи использования нейросети (кэш)",
				zap.Int64("user_id", userID),
				zap.String("model", modelName),
				zap.Error(err))
			return nil, fmt.Errorf("ошибка записи использования нейросети: %w", err)
		}

		return usage, nil
	}

	// Если запрос не найден в кэше, проверяем баланс и списываем нейроны
	if neuronsCost > 0 {
		balance, err := s.repo.GetBalance(ctx, userID)
		if err != nil {
			return nil, err
		}

		if balance.Balance < neuronsCost {
			return nil, errors.New("недостаточно нейронов для выполнения запроса")
		}

		// Списываем нейроны
		tx := &Transaction{
			UserID:          userID,
			Amount:          -neuronsCost,
			TransactionType: TypeUsage,
			Description:     fmt.Sprintf("Использование нейросети %s (%d нейронов)", modelName, neuronsCost),
			Metadata: Metadata{
				"model":             modelName,
				"prompt_tokens":     promptTokens,
				"completion_tokens": completionTokens,
			},
		}

		err = s.repo.AddTransaction(ctx, tx)
		if err != nil {
			s.log.Error("Ошибка списания нейронов за использование нейросети",
				zap.Int64("user_id", userID),
				zap.String("model", modelName),
				zap.Int("cost", neuronsCost),
				zap.Error(err))
			return nil, fmt.Errorf("ошибка списания нейронов: %w", err)
		}

		// Создаем запись использования нейросети
		usage := &LLMUsage{
			UserID:           userID,
			ModelName:        modelName,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			NeuronsCost:      neuronsCost,
			TransactionID:    &tx.ID,
			RequestHash:      requestHash,
			RequestText:      requestText,
			ResponseText:     responseText,
			Metadata:         metadata,
		}

		err = s.repo.AddLLMUsage(ctx, usage)
		if err != nil {
			s.log.Error("Ошибка записи использования нейросети",
				zap.Int64("user_id", userID),
				zap.String("model", modelName),
				zap.Error(err))
			return nil, fmt.Errorf("ошибка записи использования нейросети: %w", err)
		}

		s.log.Info("Записано использование нейросети",
			zap.Int64("user_id", userID),
			zap.String("model", modelName),
			zap.Int("cost", neuronsCost),
			zap.Int("prompt_tokens", promptTokens),
			zap.Int("completion_tokens", completionTokens))

		return usage, nil
	}

	// Если нет стоимости, просто записываем использование
	usage := &LLMUsage{
		UserID:           userID,
		ModelName:        modelName,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		NeuronsCost:      0,
		RequestHash:      requestHash,
		RequestText:      requestText,
		ResponseText:     responseText,
		Metadata:         metadata,
	}

	err = s.repo.AddLLMUsage(ctx, usage)
	if err != nil {
		s.log.Error("Ошибка записи использования нейросети (без списания)",
			zap.Int64("user_id", userID),
			zap.String("model", modelName),
			zap.Error(err))
		return nil, fmt.Errorf("ошибка записи использования нейросети: %w", err)
	}

	return usage, nil
}

// generateRequestHash генерирует хэш запроса для кэширования
func (s *Service) generateRequestHash(requestText, modelName string) string {
	// Создаем хэш на основе текста запроса и названия модели
	hasher := sha256.New()
	hasher.Write([]byte(requestText))
	hasher.Write([]byte(modelName))
	hash := hasher.Sum(nil)
	return hex.EncodeToString(hash)
}

// GetUserLLMUsageStatistics получает статистику использования нейросетей пользователем
func (s *Service) GetUserLLMUsageStatistics(ctx context.Context, userID int64) (map[string]int, error) {
	stats, err := s.repo.GetUserLLMUsageStatistics(ctx, userID)
	if err != nil {
		s.log.Error("Ошибка получения статистики использования нейросетей",
			zap.Int64("user_id", userID),
			zap.Error(err))
		return nil, fmt.Errorf("ошибка получения статистики использования нейросетей: %w", err)
	}
	return stats, nil
}

// ProcessExpiredNeurons обрабатывает истекшие нейроны
func (s *Service) ProcessExpiredNeurons(ctx context.Context) error {
	count, err := s.repo.ExpireNeurons(ctx)
	if err != nil {
		s.log.Error("Ошибка обработки истекших нейронов", zap.Error(err))
		return fmt.Errorf("ошибка обработки истекших нейронов: %w", err)
	}

	s.log.Info("Обработаны истекшие нейроны", zap.Int("user_count", count))
	return nil
}
