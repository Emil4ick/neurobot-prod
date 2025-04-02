// Сервис подписок

package subscription

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// Service предоставляет методы для работы с подписками
type Service struct {
	repo *Repository
	log  *zap.Logger
}

// NewService создает новый сервис подписок
func NewService(repo *Repository, log *zap.Logger) *Service {
	return &Service{
		repo: repo,
		log:  log.Named("subscription_service"),
	}
}

// GetAllPlans возвращает все доступные планы подписок
func (s *Service) GetAllPlans(ctx context.Context) ([]*Plan, error) {
	plans, err := s.repo.GetAllPlans(ctx)
	if err != nil {
		s.log.Error("Ошибка получения планов подписок", zap.Error(err))
		return nil, fmt.Errorf("ошибка получения планов подписок: %w", err)
	}
	return plans, nil
}

// GetPlanByCode возвращает план подписки по его коду
func (s *Service) GetPlanByCode(ctx context.Context, code string) (*Plan, error) {
	plan, err := s.repo.GetPlanByCode(ctx, code)
	if err != nil {
		s.log.Error("Ошибка получения плана подписки",
			zap.String("code", code),
			zap.Error(err))
		return nil, fmt.Errorf("ошибка получения плана подписки: %w", err)
	}

	if plan == nil {
		return nil, errors.New("план подписки не найден")
	}

	return plan, nil
}

// GetActiveSubscription получает активную подписку пользователя
func (s *Service) GetActiveSubscription(ctx context.Context, userID int64) (*Subscription, error) {
	sub, err := s.repo.GetActiveSubscription(ctx, userID)
	if err != nil {
		s.log.Error("Ошибка получения активной подписки",
			zap.Int64("user_id", userID),
			zap.Error(err))
		return nil, fmt.Errorf("ошибка получения активной подписки: %w", err)
	}
	return sub, nil
}

// SubscriptionRequest содержит данные для создания подписки
type SubscriptionRequest struct {
	UserID        int64
	PlanCode      string
	Period        string // "monthly" или "yearly"
	PaymentID     string
	PaymentMethod string
}

// Subscribe создает новую подписку
func (s *Service) Subscribe(ctx context.Context, req SubscriptionRequest) (*Subscription, error) {
	// Получаем план подписки
	plan, err := s.repo.GetPlanByCode(ctx, req.PlanCode)
	if err != nil {
		s.log.Error("Ошибка получения плана подписки",
			zap.String("plan_code", req.PlanCode),
			zap.Error(err))
		return nil, fmt.Errorf("ошибка получения плана подписки: %w", err)
	}

	if plan == nil {
		return nil, errors.New("план подписки не найден")
	}

	if !plan.IsActive {
		return nil, errors.New("план подписки не активен")
	}

	// Определение периода подписки
	startDate := time.Now()
	var endDate time.Time

	if req.Period == "yearly" {
		endDate = startDate.AddDate(1, 0, 0) // +1 год
	} else {
		endDate = startDate.AddDate(0, 1, 0) // +1 месяц
	}

	// Создаем подписку
	sub := &Subscription{
		UserID:        req.UserID,
		PlanID:        plan.ID,
		Status:        StatusActive,
		StartDate:     startDate,
		EndDate:       endDate,
		AutoRenew:     true, // По умолчанию включаем автопродление
		PaymentID:     req.PaymentID,
		PaymentMethod: req.PaymentMethod,
		Plan:          plan,
	}

	err = s.repo.CreateSubscription(ctx, sub)
	if err != nil {
		s.log.Error("Ошибка создания подписки",
			zap.Int64("user_id", req.UserID),
			zap.String("plan_code", req.PlanCode),
			zap.Error(err))
		return nil, fmt.Errorf("ошибка создания подписки: %w", err)
	}

	s.log.Info("Создана новая подписка",
		zap.Int64("user_id", req.UserID),
		zap.String("plan", plan.Name),
		zap.String("period", req.Period),
		zap.Time("end_date", endDate))

	return sub, nil
}

// CancelSubscription отменяет подписку
func (s *Service) CancelSubscription(ctx context.Context, userID, subscriptionID int64) error {
	err := s.repo.CancelSubscription(ctx, subscriptionID, userID)
	if err != nil {
		s.log.Error("Ошибка отмены подписки",
			zap.Int64("user_id", userID),
			zap.Int64("subscription_id", subscriptionID),
			zap.Error(err))
		return fmt.Errorf("ошибка отмены подписки: %w", err)
	}

	s.log.Info("Подписка отменена",
		zap.Int64("user_id", userID),
		zap.Int64("subscription_id", subscriptionID))

	return nil
}

// GetSubscriptionHistory получает историю подписок пользователя
func (s *Service) GetSubscriptionHistory(ctx context.Context, userID int64, limit, offset int) ([]*Subscription, error) {
	subs, err := s.repo.GetSubscriptionHistory(ctx, userID, limit, offset)
	if err != nil {
		s.log.Error("Ошибка получения истории подписок",
			zap.Int64("user_id", userID),
			zap.Error(err))
		return nil, fmt.Errorf("ошибка получения истории подписок: %w", err)
	}
	return subs, nil
}

// HasActiveSubscription проверяет, есть ли у пользователя активная подписка
func (s *Service) HasActiveSubscription(ctx context.Context, userID int64) (bool, error) {
	sub, err := s.repo.GetActiveSubscription(ctx, userID)
	if err != nil {
		return false, err
	}
	return sub != nil, nil
}

// HasPremiumSubscription проверяет, есть ли у пользователя премиум подписка
func (s *Service) HasPremiumSubscription(ctx context.Context, userID int64) (bool, error) {
	sub, err := s.repo.GetActiveSubscription(ctx, userID)
	if err != nil {
		return false, err
	}

	if sub == nil || sub.Plan == nil {
		return false, nil
	}

	// Проверяем, что подписка не бесплатная
	return sub.Plan.Code != "free", nil
}

// GetSubscriptionPlan получает план активной подписки пользователя
func (s *Service) GetSubscriptionPlan(ctx context.Context, userID int64) (*Plan, error) {
	sub, err := s.repo.GetActiveSubscription(ctx, userID)
	if err != nil {
		return nil, err
	}

	if sub == nil || sub.Plan == nil {
		// Если нет активной подписки, возвращаем бесплатный план
		return s.repo.GetPlanByCode(ctx, "free")
	}

	return sub.Plan, nil
}

// ProcessExpiringSubscriptions обрабатывает подписки, которые истекают в ближайшем будущем
func (s *Service) ProcessExpiringSubscriptions(ctx context.Context, daysThreshold int) error {
	// Сначала помечаем просроченные подписки как истекшие
	expiredCount, err := s.repo.ExpireSubscriptions(ctx)
	if err != nil {
		s.log.Error("Ошибка обработки просроченных подписок", zap.Error(err))
		return fmt.Errorf("ошибка обработки просроченных подписок: %w", err)
	}

	s.log.Info("Обработаны просроченные подписки", zap.Int("expired_count", expiredCount))

	// Затем получаем подписки, которые скоро истекут
	expiringSubscriptions, err := s.repo.GetExpiringSubscriptions(ctx, daysThreshold)
	if err != nil {
		s.log.Error("Ошибка получения истекающих подписок", zap.Error(err))
		return fmt.Errorf("ошибка получения истекающих подписок: %w", err)
	}

	s.log.Info("Найдены подписки, истекающие в ближайшие дни",
		zap.Int("days_threshold", daysThreshold),
		zap.Int("count", len(expiringSubscriptions)))

	// Здесь можно добавить логику для отправки уведомлений пользователям
	// или автоматического продления подписок

	return nil
}

// GetDailyNeurons возвращает количество ежедневных нейронов для пользователя
func (s *Service) GetDailyNeurons(ctx context.Context, userID int64, loyaltyBonusPercent int) (int, error) {
	sub, err := s.repo.GetActiveSubscription(ctx, userID)
	if err != nil {
		return 0, err
	}

	if sub == nil || sub.Plan == nil {
		// Если нет активной подписки, возвращаем значение для бесплатного плана
		freePlan, err := s.repo.GetPlanByCode(ctx, "free")
		if err != nil {
			return 0, err
		}

		if freePlan == nil {
			return 0, errors.New("бесплатный план не найден")
		}

		dailyNeurons := freePlan.DailyNeurons
		if loyaltyBonusPercent > 0 {
			bonusNeurons := dailyNeurons * loyaltyBonusPercent / 100
			dailyNeurons += bonusNeurons
		}

		return dailyNeurons, nil
	}

	// Для существующей подписки
	dailyNeurons := sub.Plan.DailyNeurons
	if loyaltyBonusPercent > 0 {
		bonusNeurons := dailyNeurons * loyaltyBonusPercent / 100
		dailyNeurons += bonusNeurons
	}

	return dailyNeurons, nil
}
