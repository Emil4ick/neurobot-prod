package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"neurobot-prod/internal/config"
	"neurobot-prod/internal/currency"
	"neurobot-prod/internal/llm"
	"neurobot-prod/internal/queue"
	"neurobot-prod/internal/storage/postgres"
	redisStorage "neurobot-prod/internal/storage/redis"
	"neurobot-prod/internal/subscription"
	"neurobot-prod/internal/telegram"
)

// MessageWorker обрабатывает сообщения от пользователей
type MessageWorker struct {
	db               *sql.DB
	redis            *redis.Client
	bot              *telegram.Bot
	natsConn         *nats.Conn
	natsSubscription *nats.Subscription
	publisher        *queue.Publisher
	config           *config.Config
	log              *zap.Logger
	currencyService  *currency.Service
	subService       *subscription.Service
	llmService       *llm.Service
}

// NewMessageWorker создает новый обработчик сообщений
func NewMessageWorker(cfg *config.Config, log *zap.Logger) (*MessageWorker, error) {
	logger := log.Named("message_worker")

	// Подключаемся к базе данных
	db, err := postgres.NewPostgresDB(cfg.DB, logger)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к базе данных: %w", err)
	}

	// Подключаемся к Redis
	redisClient, err := redisStorage.NewRedisClient(cfg.Redis, logger)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к Redis: %w", err)
	}

	// Создаем бота
	bot, err := telegram.NewBot(cfg.Telegram, logger)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания Telegram бота: %w", err)
	}

	// Настраиваем NATS
	natsOpts := []nats.Option{
		nats.Name("Neurobot Message Worker"),
		nats.ReconnectWait(cfg.NATS.GetReconnectWait()),
		nats.MaxReconnects(cfg.NATS.MaxReconnects),
		nats.Timeout(cfg.NATS.GetTimeout()),
	}

	natsConn, err := nats.Connect(cfg.NATS.URL, natsOpts...)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к NATS: %w", err)
	}

	// Создаем NATS Publisher
	publisher, err := queue.NewPublisher(cfg.NATS, logger)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания NATS publisher: %w", err)
	}

	// Создаем репозитории
	currencyRepo := currency.NewRepository(db)
	subRepo := subscription.NewRepository(db)

	// Создаем сервисы
	subService := subscription.NewService(subRepo, logger)
	currencyService := currency.NewService(currencyRepo, subService, logger)
	llmService := llm.NewService(cfg, subService, currencyService, logger)

	return &MessageWorker{
		db:              db,
		redis:           redisClient,
		bot:             bot,
		natsConn:        natsConn,
		publisher:       publisher,
		config:          cfg,
		log:             logger,
		currencyService: currencyService,
		subService:      subService,
		llmService:      llmService,
	}, nil
}

// Start запускает обработчик сообщений
func (w *MessageWorker) Start() error {
	// Подписываемся на очередь сообщений
	sub, err := w.natsConn.Subscribe(w.config.NATS.Subjects.TelegramUpdates, w.handleMessage)
	if err != nil {
		return fmt.Errorf("ошибка подписки на тему NATS: %w", err)
	}

	w.natsSubscription = sub
	w.log.Info("Начало обработки сообщений",
		zap.String("subject", w.config.NATS.Subjects.TelegramUpdates))

	return nil
}

// Stop останавливает обработчик сообщений
func (w *MessageWorker) Stop() {
	// Отписываемся от NATS
	if w.natsSubscription != nil {
		w.natsSubscription.Unsubscribe()
	}

	// Закрываем соединение с NATS
	if w.natsConn != nil {
		w.natsConn.Close()
	}

	// Закрываем соединение с базой данных
	if w.db != nil {
		w.db.Close()
	}

	// Закрываем соединение с Redis
	if w.redis != nil {
		w.redis.Close()
	}

	w.log.Info("Обработчик сообщений остановлен")
}

// handleMessage обрабатывает сообщение из NATS
func (w *MessageWorker) handleMessage(msg *nats.Msg) {
	// Парсим обновление
	var update tgbotapi.Update
	if err := json.Unmarshal(msg.Data, &update); err != nil {
		w.log.Error("Ошибка парсинга обновления", zap.Error(err))
		return
	}

	// Обрабатываем обновление в зависимости от его типа
	switch {
	case update.Message != nil:
		w.handleTextMessage(&update)
	case update.CallbackQuery != nil:
		w.handleCallbackQuery(&update)
	default:
		w.log.Debug("Получен неподдерживаемый тип обновления")
	}
}

// handleTextMessage обрабатывает текстовые сообщения
func (w *MessageWorker) handleTextMessage(update *tgbotapi.Update) {
	message := update.Message

	// Обрабатываем только текстовые сообщения
	if message.Text == "" {
		return
	}

	// Проверяем, является ли сообщение командой
	if message.IsCommand() {
		w.handleCommand(message)
		return
	}

	// Здесь обрабатываем обычные текстовые сообщения (запросы к нейросети)
	w.handleNeuralRequest(message)
}

// handleCommand обрабатывает команды
func (w *MessageWorker) handleCommand(message *tgbotapi.Message) {
	command := message.Command()

	switch command {
	case "start":
		w.handleStartCommand(message)
	case "help":
		w.handleHelpCommand(message)
	case "profile":
		w.handleProfileCommand(message)
	case "daily":
		w.handleDailyCommand(message)
	case "models":
		w.handleModelsCommand(message)
	case "subscribe":
		w.handleSubscribeCommand(message)
	default:
		w.bot.SendMessage(message.Chat.ID, "Неизвестная команда. Введите /help для получения списка доступных команд.")
	}
}

// handleStartCommand обрабатывает команду /start
func (w *MessageWorker) handleStartCommand(message *tgbotapi.Message) {
	// Приветственное сообщение
	text := fmt.Sprintf(
		"Привет, %s! 👋\n\n"+
			"Я бот *Твоя Нейросеть* - твой помощник с доступом к самым современным нейросетям, таким как ChatGPT, Claude, Grok и Gemini.\n\n"+
			"Для работы я использую внутреннюю валюту - *Нейроны*. Ты получаешь бесплатные нейроны каждый день, а также можешь приобрести их дополнительно.\n\n"+
			"Просто напиши мне свой вопрос, и я отправлю его нейросети!\n\n"+
			"Команды:\n"+
			"/daily - получить ежедневные нейроны\n"+
			"/profile - информация о профиле\n"+
			"/models - доступные модели нейросетей\n"+
			"/subscribe - информация о подписках\n"+
			"/help - справка по командам",
		message.From.FirstName)

	// Отправляем приветственное сообщение с кнопкой для открытия Mini App
	w.bot.SendMessage(message.Chat.ID, text,
		telegram.WithParseMode("Markdown"),
		telegram.WithWebAppInfo())
}

// handleHelpCommand обрабатывает команду /help
func (w *MessageWorker) handleHelpCommand(message *tgbotapi.Message) {
	text := "Список доступных команд:\n\n" +
		"/start - начало работы с ботом\n" +
		"/daily - получить ежедневные нейроны\n" +
		"/profile - информация о профиле\n" +
		"/models - доступные модели нейросетей\n" +
		"/subscribe - информация о подписках\n" +
		"/help - справка по командам\n\n" +
		"Для взаимодействия с нейросетью просто отправь мне свой вопрос!"

	w.bot.SendMessage(message.Chat.ID, text)
}

// handleProfileCommand обрабатывает команду /profile
func (w *MessageWorker) handleProfileCommand(message *tgbotapi.Message) {
	// Получаем баланс пользователя
	balance, err := w.currencyService.GetBalance(context.Background(), int64(message.From.ID))
	if err != nil {
		w.log.Error("Ошибка получения баланса",
			zap.Int64("user_id", int64(message.From.ID)),
			zap.Error(err))
		w.bot.SendMessage(message.Chat.ID, "Произошла ошибка при получении информации о профиле. Попробуйте позже.")
		return
	}

	// Получаем активную подписку
	subscription, err := w.subService.GetActiveSubscription(context.Background(), int64(message.From.ID))
	if err != nil {
		w.log.Error("Ошибка получения подписки",
			zap.Int64("user_id", int64(message.From.ID)),
			zap.Error(err))
		w.bot.SendMessage(message.Chat.ID, "Произошла ошибка при получении информации о подписке. Попробуйте позже.")
		return
	}

	// Формируем текст профиля
	var subscriptionInfo string
	if subscription != nil && subscription.Plan != nil {
		if subscription.IsFree() {
			subscriptionInfo = "Бесплатный"
		} else {
			daysLeft := subscription.DaysLeft()
			subscriptionInfo = fmt.Sprintf("%s (осталось дней: %d)", subscription.Plan.Name, daysLeft)
		}
	} else {
		subscriptionInfo = "Бесплатный"
	}

	text := fmt.Sprintf(
		"*Профиль пользователя*\n\n"+
			"🧠 *Нейроны:* %d\n"+
			"📊 *Всего получено:* %d\n"+
			"📉 *Всего потрачено:* %d\n"+
			"🔎 *Тип подписки:* %s\n\n"+
			"Для просмотра полной информации о профиле, включая достижения и историю транзакций, нажмите кнопку \"Открыть Профиль\" ниже.",
		balance.Balance,
		balance.LifetimeEarned,
		balance.LifetimeSpent,
		subscriptionInfo)

	w.bot.SendMessage(message.Chat.ID, text,
		telegram.WithParseMode("Markdown"),
		telegram.WithWebAppInfo())
}

// handleDailyCommand обрабатывает команду /daily
func (w *MessageWorker) handleDailyCommand(message *tgbotapi.Message) {
	// Получаем баланс пользователя для проверки
	balance, err := w.currencyService.GetBalance(context.Background(), int64(message.From.ID))
	if err != nil {
		w.log.Error("Ошибка получения баланса",
			zap.Int64("user_id", int64(message.From.ID)),
			zap.Error(err))
		w.bot.SendMessage(message.Chat.ID, "Произошла ошибка при получении информации о балансе. Попробуйте позже.")
		return
	}

	// Проверяем, может ли пользователь получить ежедневное вознаграждение
	if !balance.CanReceiveDailyReward() {
		timeLeft := time.Until(*balance.LastDailyRewardAt).Add(20 * time.Hour)
		hours := int(timeLeft.Hours())
		minutes := int(timeLeft.Minutes()) % 60

		w.bot.SendMessage(message.Chat.ID,
			fmt.Sprintf("Вы уже получили ежедневные нейроны. Следующее начисление будет доступно через %d ч %d мин.",
				hours, minutes))
		return
	}

	// Получаем реферальный бонус
	// TODO: Реализовать логику получения реферального бонуса
	loyaltyBonusPercent := 0

	// Добавляем ежедневные нейроны
	tx, err := w.currencyService.AddDailyNeurons(context.Background(), int64(message.From.ID), loyaltyBonusPercent)
	if err != nil {
		w.log.Error("Ошибка начисления ежедневных нейронов",
			zap.Int64("user_id", int64(message.From.ID)),
			zap.Error(err))
		w.bot.SendMessage(message.Chat.ID, "Произошла ошибка при начислении ежедневных нейронов. Попробуйте позже.")
		return
	}

	// Уведомляем пользователя о начислении
	text := fmt.Sprintf(
		"✅ *Ежедневное начисление нейронов*\n\n"+
			"Вам начислено *%d нейронов*!\n"+
			"Текущий баланс: *%d нейронов*\n\n"+
			"Эти нейроны будут действовать в течение ограниченного периода времени. Используйте их для запросов к нейросетям!",
		tx.Amount,
		tx.BalanceAfter)

	w.bot.SendMessage(message.Chat.ID, text, telegram.WithParseMode("Markdown"))
}

// handleModelsCommand обрабатывает команду /models
func (w *MessageWorker) handleModelsCommand(message *tgbotapi.Message) {
	// Получаем доступные модели для пользователя
	models, err := w.llmService.GetAvailableModels(context.Background(), int64(message.From.ID))
	if err != nil {
		w.log.Error("Ошибка получения доступных моделей",
			zap.Int64("user_id", int64(message.From.ID)),
			zap.Error(err))
		w.bot.SendMessage(message.Chat.ID, "Произошла ошибка при получении списка доступных моделей. Попробуйте позже.")
		return
	}

	// Группируем модели по типу
	modelsByType := make(map[llm.ModelType][]llm.ModelConfig)
	for _, model := range models {
		modelsByType[model.Type] = append(modelsByType[model.Type], model)
	}

	// Формируем текст со списком моделей
	var parts []string
	parts = append(parts, "*Доступные модели нейросетей:*\n")

	// Добавляем OpenAI модели
	if openaiModels, ok := modelsByType[llm.ModelTypeOpenAI]; ok && len(openaiModels) > 0 {
		parts = append(parts, "*ChatGPT (OpenAI):*")
		for _, model := range openaiModels {
			parts = append(parts, fmt.Sprintf("• %s - %s (%d нейронов)",
				model.DisplayName, model.Description, model.NeuronsCost))
		}
		parts = append(parts, "")
	}

	// Добавляем Claude модели
	if claudeModels, ok := modelsByType[llm.ModelTypeClaude]; ok && len(claudeModels) > 0 {
		parts = append(parts, "*Claude (Anthropic):*")
		for _, model := range claudeModels {
			parts = append(parts, fmt.Sprintf("• %s - %s (%d нейронов)",
				model.DisplayName, model.Description, model.NeuronsCost))
		}
		parts = append(parts, "")
	}

	// Добавляем Grok модели
	if grokModels, ok := modelsByType[llm.ModelTypeGrok]; ok && len(grokModels) > 0 {
		parts = append(parts, "*Grok (xAI):*")
		for _, model := range grokModels {
			parts = append(parts, fmt.Sprintf("• %s - %s (%d нейронов)",
				model.DisplayName, model.Description, model.NeuronsCost))
		}
		parts = append(parts, "")
	}

	// Добавляем Gemini модели
	if geminiModels, ok := modelsByType[llm.ModelTypeGemini]; ok && len(geminiModels) > 0 {
		parts = append(parts, "*Gemini (Google):*")
		for _, model := range geminiModels {
			parts = append(parts, fmt.Sprintf("• %s - %s (%d нейронов)",
				model.DisplayName, model.Description, model.NeuronsCost))
		}
		parts = append(parts, "")
	}

	// Добавляем информацию о подписке
	parts = append(parts, "Для доступа к продвинутым моделям нейросетей оформите подписку Premium или Pro через команду /subscribe.")

	// Отправляем сообщение
	text := strings.Join(parts, "\n")
	w.bot.SendMessage(message.Chat.ID, text, telegram.WithParseMode("Markdown"))
}

// handleSubscribeCommand обрабатывает команду /subscribe
func (w *MessageWorker) handleSubscribeCommand(message *tgbotapi.Message) {
	// Получаем все планы подписок
	plans, err := w.subService.GetAllPlans(context.Background())
	if err != nil {
		w.log.Error("Ошибка получения планов подписок",
			zap.Int64("user_id", int64(message.From.ID)),
			zap.Error(err))
		w.bot.SendMessage(message.Chat.ID, "Произошла ошибка при получении информации о подписках. Попробуйте позже.")
		return
	}

	// Формируем текст со списком подписок
	var parts []string
	parts = append(parts, "*Доступные подписки:*\n")

	for _, plan := range plans {
		if plan.Code == "free" {
			parts = append(parts, fmt.Sprintf("*%s*\n• Цена: Бесплатно\n• Ежедневные нейроны: %d\n• Максимальная длина запроса: %d символов\n",
				plan.Name, plan.DailyNeurons, plan.MaxRequestLength))
		} else {
			parts = append(parts, fmt.Sprintf("*%s*\n• Цена: %.2f ₽/мес или %.2f ₽/год (экономия %d%%)\n• Ежедневные нейроны: %d\n• Максимальная длина запроса: %d символов\n• Бонус при подписке: %d нейронов\n",
				plan.Name, plan.GetMonthlyPriceRub(), plan.GetYearlyPriceRub(), plan.GetYearlySavingPercent(),
				plan.DailyNeurons, plan.MaxRequestLength, plan.GetWelcomeBonus()))
		}
	}

	parts = append(parts, "Для оформления подписки и просмотра всех преимуществ, используйте кнопку \"Открыть Профиль\" ниже.")

	// Отправляем сообщение
	text := strings.Join(parts, "\n")
	w.bot.SendMessage(message.Chat.ID, text,
		telegram.WithParseMode("Markdown"),
		telegram.WithWebAppInfo())
}

// handleCallbackQuery обрабатывает callback-запросы (нажатия на инлайн-кнопки)
func (w *MessageWorker) handleCallbackQuery(update *tgbotapi.Update) {
	// TODO: Реализовать обработку callback-запросов
}

// handleNeuralRequest обрабатывает запрос к нейросети
func (w *MessageWorker) handleNeuralRequest(message *tgbotapi.Message) {
	userID := int64(message.From.ID)
	messageText := message.Text
	chatID := message.Chat.ID

	// Отправляем уведомление о том, что запрос обрабатывается
	responseMsg, err := w.bot.SendMessage(chatID, "⏳ Обрабатываю ваш запрос...")
	if err != nil {
		w.log.Error("Ошибка отправки уведомления",
			zap.Int64("user_id", userID),
			zap.Error(err))
		return
	}

	// Получаем план подписки пользователя
	plan, err := w.subService.GetSubscriptionPlan(context.Background(), userID)
	if err != nil {
		w.log.Error("Ошибка получения плана подписки",
			zap.Int64("user_id", userID),
			zap.Error(err))
		w.bot.SendMessage(chatID, "Произошла ошибка при проверке подписки. Попробуйте позже.")
		return
	}

	// Проверяем длину запроса
	if plan.MaxRequestLength > 0 && len([]rune(messageText)) > plan.MaxRequestLength {
		w.bot.SendMessage(chatID, fmt.Sprintf(
			"❌ Слишком длинный запрос!\n\nМаксимальная длина запроса для вашего плана подписки: %d символов.\n"+
				"Длина вашего запроса: %d символов.\n\n"+
				"Сократите запрос или оформите подписку с увеличенным лимитом через /subscribe.",
			plan.MaxRequestLength,
			len([]rune(messageText))))
		return
	}

	// Проверяем баланс нейронов
	balance, err := w.currencyService.GetBalance(context.Background(), userID)
	if err != nil {
		w.log.Error("Ошибка получения баланса",
			zap.Int64("user_id", userID),
			zap.Error(err))
		w.bot.SendMessage(chatID, "Произошла ошибка при проверке баланса нейронов. Попробуйте позже.")
		return
	}

	// Получаем базовую модель для запроса
	availableModels, err := w.llmService.GetAvailableModels(context.Background(), userID)
	if err != nil {
		w.log.Error("Ошибка получения доступных моделей",
			zap.Int64("user_id", userID),
			zap.Error(err))
		w.bot.SendMessage(chatID, "Произошла ошибка при определении доступных моделей. Попробуйте позже.")
		return
	}

	// Выбираем модель (для примера, берем первую доступную)
	if len(availableModels) == 0 {
		w.bot.SendMessage(chatID, "У вас нет доступных моделей нейросетей. Пожалуйста, обратитесь в поддержку.")
		return
	}

	// Выбираем модель GPT-3.5 Turbo или Claude Haiku (базовые)
	var selectedModel llm.ModelConfig
	for _, model := range availableModels {
		if model.Name == "gpt-3.5-turbo" {
			selectedModel = model
			break
		} else if model.Name == "claude-3-haiku-20240307" {
			selectedModel = model
			break
		}
	}

	// Если модели не найдены, берем первую доступную
	if selectedModel.Name == "" {
		selectedModel = availableModels[0]
	}

	// Проверяем, достаточно ли нейронов
	if balance.Balance < selectedModel.NeuronsCost {
		w.bot.SendMessage(chatID, fmt.Sprintf(
			"❌ Недостаточно нейронов для запроса!\n\n"+
				"Стоимость запроса: %d нейронов\n"+
				"Ваш баланс: %d нейронов\n\n"+
				"Получите ежедневное начисление через /daily или приобретите дополнительные нейроны через \"Открыть Профиль\".",
			selectedModel.NeuronsCost,
			balance.Balance))
		return
	}

	// Создаем запрос к нейросети
	llmRequest := &llm.Request{
		UserID:      userID,
		UserMessage: messageText,
		ModelType:   selectedModel.Type,
		ModelName:   selectedModel.Name,
		// Для бесплатных пользователей не добавляем историю сообщений
		MessageHistory: []llm.Message{},
	}

	// Отправляем запрос к нейросети
	response, err := w.llmService.ProcessRequest(context.Background(), llmRequest)
	if err != nil {
		w.log.Error("Ошибка обработки запроса к нейросети",
			zap.Int64("user_id", userID),
			zap.Error(err))
		w.bot.SendMessage(chatID, "Произошла ошибка при обработке запроса к нейросети. Попробуйте позже.")
		return
	}

	// Формируем подпись с информацией о модели и стоимости
	footer := fmt.Sprintf("\n\n---\n📊 Модель: %s\n💰 Стоимость: %d нейронов",
		selectedModel.DisplayName, response.NeuronsCost)

	// Если ответ был из кэша, добавляем информацию
	if response.Cached {
		footer += " (ответ из кэша)"
	}

	// Отправляем результат (обновляем существующее сообщение)
	limitedResponse := response.ResponseText
	// Ограничиваем длину ответа, если необходимо
	if len(limitedResponse) > 4000 {
		limitedResponse = limitedResponse[:4000] + "...\n\n(Ответ был слишком длинным и был обрезан)"
	}

	// Отправляем готовый ответ, обновляя предыдущее сообщение с "обрабатываю запрос"
	w.bot.SendMessage(chatID, limitedResponse+footer)
}
