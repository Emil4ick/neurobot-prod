// Сервис для работы с нейросетями

package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"neurobot-prod/internal/config"
	"neurobot-prod/internal/currency"
	"neurobot-prod/internal/subscription"
)

// ClientInterface определяет интерфейс для клиентов нейросетей
type ClientInterface interface {
	ProcessRequest(ctx context.Context, request *Request) (*Response, error)
	GetModelInfo() []ModelConfig
}

// Service предоставляет методы для работы с нейросетями
type Service struct {
	config        *config.Config
	clients       map[ModelType]ClientInterface
	subService    *subscription.Service
	neuronService *currency.Service
	log           *zap.Logger
}

// NewService создает новый сервис для работы с нейросетями
func NewService(cfg *config.Config, subService *subscription.Service, neuronService *currency.Service, log *zap.Logger) *Service {
	service := &Service{
		config:        cfg,
		clients:       make(map[ModelType]ClientInterface),
		subService:    subService,
		neuronService: neuronService,
		log:           log.Named("llm_service"),
	}

	// Инициализируем клиенты для разных типов нейросетей
	if cfg.LLM.OpenAI.ApiKey != "" {
		service.clients[ModelTypeOpenAI] = NewOpenAIClient(cfg.LLM.OpenAI, log)
	}

	if cfg.LLM.Claude.ApiKey != "" {
		service.clients[ModelTypeClaude] = NewClaudeClient(cfg.LLM.Claude, log)
	}

	if cfg.LLM.Grok.ApiKey != "" {
		service.clients[ModelTypeGrok] = NewGrokClient(cfg.LLM.Grok, log)
	}

	if cfg.LLM.Gemini.ApiKey != "" {
		service.clients[ModelTypeGemini] = NewGeminiClient(cfg.LLM.Gemini, log)
	}

	return service
}

// ProcessRequest обрабатывает запрос к нейросети
func (s *Service) ProcessRequest(ctx context.Context, request *Request) (*Response, error) {
	// Проверяем, что клиент для указанного типа модели существует
	client, ok := s.clients[request.ModelType]
	if !ok {
		s.log.Error("Клиент для указанного типа модели не найден",
			zap.String("model_type", string(request.ModelType)))
		return nil, fmt.Errorf("клиент для типа модели %s не найден", request.ModelType)
	}

	// Проверяем наличие активной подписки и возможность использования модели
	hasAccess, err := s.CheckModelAccess(ctx, request.UserID, request.ModelType, request.ModelName)
	if err != nil {
		return nil, err
	}

	if !hasAccess {
		return nil, errors.New("нет доступа к выбранной модели")
	}

	// Проверяем длину запроса в зависимости от подписки
	if err := s.CheckRequestLength(ctx, request.UserID, request.UserMessage); err != nil {
		return nil, err
	}

	// Получаем стоимость запроса в нейронах
	cost, err := s.GetRequestCost(ctx, request.ModelType, request.ModelName)
	if err != nil {
		return nil, err
	}

	// Проверяем наличие достаточного количества нейронов
	hasEnough, err := s.neuronService.HasEnoughNeurons(ctx, request.UserID, cost)
	if err != nil {
		return nil, err
	}

	if !hasEnough {
		return nil, errors.New("недостаточно нейронов для выполнения запроса")
	}

	// Выполняем запрос к нейросети
	response, err := client.ProcessRequest(ctx, request)
	if err != nil {
		s.log.Error("Ошибка выполнения запроса к нейросети",
			zap.Int64("user_id", request.UserID),
			zap.String("model_type", string(request.ModelType)),
			zap.String("model_name", request.ModelName),
			zap.Error(err))
		return nil, fmt.Errorf("ошибка выполнения запроса к нейросети: %w", err)
	}

	// Записываем использование нейросети и списываем нейроны
	metadata := map[string]interface{}{
		"model_type":        request.ModelType,
		"model_name":        request.ModelName,
		"prompt_tokens":     response.PromptTokens,
		"completion_tokens": response.CompletionTokens,
		"conversation_id":   request.ConversationID,
	}

	// Применяем скидку на нейроны в зависимости от подписки
	actualCost := s.ApplyNeuronDiscount(ctx, request.UserID, cost)
	response.NeuronsCost = actualCost

	_, err = s.neuronService.RecordLLMUsage(
		ctx,
		request.UserID,
		request.ModelName,
		request.UserMessage,
		response.ResponseText,
		response.PromptTokens,
		response.CompletionTokens,
		actualCost,
		currency.Metadata(metadata),
	)

	if err != nil {
		s.log.Error("Ошибка записи использования нейросети",
			zap.Int64("user_id", request.UserID),
			zap.String("model_name", request.ModelName),
			zap.Error(err))
		// Не возвращаем ошибку, так как запрос уже выполнен
	}

	s.log.Info("Запрос к нейросети выполнен успешно",
		zap.Int64("user_id", request.UserID),
		zap.String("model_type", string(request.ModelType)),
		zap.String("model_name", request.ModelName),
		zap.Int("prompt_tokens", response.PromptTokens),
		zap.Int("completion_tokens", response.CompletionTokens),
		zap.Int("neurons_cost", actualCost))

	return response, nil
}

// GetAvailableModels возвращает список доступных моделей для пользователя
func (s *Service) GetAvailableModels(ctx context.Context, userID int64) ([]ModelConfig, error) {
	// Получаем план подписки пользователя
	plan, err := s.subService.GetSubscriptionPlan(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Получаем доступные модели из плана подписки
	availableModels := plan.GetAvailableModels()

	// Собираем все конфигурации моделей
	var allModels []ModelConfig
	for _, client := range s.clients {
		allModels = append(allModels, client.GetModelInfo()...)
	}

	// Фильтруем модели по доступности для данного плана
	var userModels []ModelConfig
	for _, model := range allModels {
		for _, availableModel := range availableModels {
			if model.Name == availableModel {
				userModels = append(userModels, model)
				break
			}
		}
	}

	return userModels, nil
}

// CheckModelAccess проверяет, имеет ли пользователь доступ к указанной модели
func (s *Service) CheckModelAccess(ctx context.Context, userID int64, modelType ModelType, modelName string) (bool, error) {
	// Получаем план подписки пользователя
	plan, err := s.subService.GetSubscriptionPlan(ctx, userID)
	if err != nil {
		return false, err
	}

	// Получаем доступные модели из плана подписки
	availableModels := plan.GetAvailableModels()

	// Проверяем, доступна ли модель
	for _, availableModel := range availableModels {
		if availableModel == modelName {
			return true, nil
		}
	}

	return false, nil
}

// CheckRequestLength проверяет, не превышает ли длина запроса максимально допустимую
func (s *Service) CheckRequestLength(ctx context.Context, userID int64, message string) error {
	// Получаем план подписки пользователя
	plan, err := s.subService.GetSubscriptionPlan(ctx, userID)
	if err != nil {
		return err
	}

	// Максимальная длина запроса для данного плана
	maxLength := plan.MaxRequestLength

	// Если максимальная длина равна 0, то ограничения нет
	if maxLength == 0 {
		return nil
	}

	// Проверяем длину запроса
	if len([]rune(message)) > maxLength {
		return fmt.Errorf("длина запроса превышает максимально допустимую (%d символов)", maxLength)
	}

	return nil
}

// GetRequestCost возвращает стоимость запроса в нейронах
func (s *Service) GetRequestCost(ctx context.Context, modelType ModelType, modelName string) (int, error) {
	// Получаем конфигурацию модели
	var cost int

	// Проверяем тип модели и получаем соответствующую стоимость
	switch {
	case strings.Contains(modelName, "gpt-4"):
		cost = s.config.Subscription.ProModelCost
	case strings.Contains(modelName, "gpt-3.5"):
		cost = s.config.Subscription.BaseModelCost
	case strings.Contains(modelName, "claude-3-opus"):
		cost = s.config.Subscription.ProModelCost
	case strings.Contains(modelName, "claude-3-sonnet"):
		cost = s.config.Subscription.PremiumModelCost
	case strings.Contains(modelName, "claude-3-haiku"):
		cost = s.config.Subscription.BaseModelCost
	case strings.Contains(modelName, "grok-2"):
		cost = s.config.Subscription.ProModelCost
	case strings.Contains(modelName, "grok-1"):
		cost = s.config.Subscription.BaseModelCost
	case strings.Contains(modelName, "gemini-1.5"):
		cost = s.config.Subscription.PremiumModelCost
	case strings.Contains(modelName, "gemini-1.0"):
		cost = s.config.Subscription.BaseModelCost
	default:
		// По умолчанию используем базовую стоимость
		cost = s.config.Subscription.BaseModelCost
	}

	return cost, nil
}

// ApplyNeuronDiscount применяет скидку на нейроны в зависимости от подписки
func (s *Service) ApplyNeuronDiscount(ctx context.Context, userID int64, cost int) int {
	// Получаем план подписки пользователя
	plan, err := s.subService.GetSubscriptionPlan(ctx, userID)
	if err != nil {
		// В случае ошибки не применяем скидку
		return cost
	}

	// Получаем скидку в процентах
	discount := plan.GetNeuronDiscount()
	if discount <= 0 {
		return cost
	}

	// Применяем скидку
	discountedCost := cost - (cost * discount / 100)
	if discountedCost < 1 {
		discountedCost = 1 // Минимальная стоимость - 1 нейрон
	}

	return discountedCost
}

// ModelMessageLimit возвращает максимальное количество сохраненных сообщений для контекста
func (s *Service) ModelMessageLimit(ctx context.Context, userID int64) (int, error) {
	// Получаем план подписки пользователя
	plan, err := s.subService.GetSubscriptionPlan(ctx, userID)
	if err != nil {
		return 0, err
	}

	return plan.ContextMessages, nil
}
