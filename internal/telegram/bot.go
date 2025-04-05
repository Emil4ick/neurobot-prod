package telegram

import (
	"fmt"
	"net/url"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"neurobot-prod/internal/config"
)

// Bot представляет собой обертку вокруг Telegram-бота API
type Bot struct {
	api        *tgbotapi.BotAPI
	config     config.TelegramConfig
	log        *zap.Logger
	webhookURL string
}

// NewBot создает новый экземпляр бота
func NewBot(cfg config.TelegramConfig, log *zap.Logger) (*Bot, error) {
	logger := log.Named("telegram_bot")

	// Проверяем, что токен установлен
	if cfg.Token == "" {
		return nil, fmt.Errorf("токен Telegram бота не установлен")
	}

	// Создаем бота
	api, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания Telegram бота: %w", err)
	}

	// В режиме разработки включаем подробное логирование
	if cfg.WebhookBaseURL != "https://yourneuro.ru" {
		api.Debug = true
		logger.Info("Включен режим отладки для Telegram API")
	}

	// Формируем URL для вебхука
	webhookURL := cfg.WebhookBaseURL + cfg.WebhookPath

	logger.Info("Telegram бот успешно создан",
		zap.String("username", api.Self.UserName),
		zap.String("webhook_url", webhookURL))

	return &Bot{
		api:        api,
		config:     cfg,
		log:        logger,
		webhookURL: webhookURL,
	}, nil
}

// SetWebhook устанавливает вебхук для бота
func (b *Bot) SetWebhook() error {
	// Преобразуем строку URL в объект *url.URL
	parsedURL, err := url.Parse(b.webhookURL)
	if err != nil {
		return fmt.Errorf("ошибка парсинга URL вебхука: %w", err)
	}

	// Создаем запрос на установку вебхука
	webhook := tgbotapi.WebhookConfig{
		URL: parsedURL,
	}

	// Устанавливаем вебхук
	_, err = b.api.Request(webhook)
	if err != nil {
		return fmt.Errorf("ошибка установки вебхука: %w", err)
	}

	// Получаем информацию о текущем вебхуке
	info, err := b.api.GetWebhookInfo()
	if err != nil {
		return fmt.Errorf("ошибка получения информации о вебхуке: %w", err)
	}

	// Проверяем, есть ли ошибки
	if info.LastErrorDate != 0 {
		lastErrorTime := time.Unix(int64(info.LastErrorDate), 0)
		b.log.Error("Ошибка вебхука",
			zap.Time("last_error_time", lastErrorTime),
			zap.String("last_error_message", info.LastErrorMessage))
	}

	b.log.Info("Вебхук успешно установлен",
		zap.String("url", b.webhookURL),
		zap.Bool("has_custom_cert", info.HasCustomCertificate))

	return nil
}

// RemoveWebhook удаляет вебхук бота
func (b *Bot) RemoveWebhook() error {
	_, err := b.api.Request(tgbotapi.DeleteWebhookConfig{})
	if err != nil {
		return fmt.Errorf("ошибка удаления вебхука: %w", err)
	}

	b.log.Info("Вебхук успешно удален")
	return nil
}

// GetUpdatesChan возвращает канал для получения обновлений (для работы без вебхука)
func (b *Bot) GetUpdatesChan() tgbotapi.UpdatesChannel {
	b.log.Info("Запуск в режиме long polling")

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	return b.api.GetUpdatesChan(u)
}

// SendMessage отправляет сообщение пользователю
func (b *Bot) SendMessage(chatID int64, text string, options ...MessageOption) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(chatID, text)

	// Применяем опции
	for _, option := range options {
		option(&msg)
	}

	sentMsg, err := b.api.Send(msg)
	if err != nil {
		return tgbotapi.Message{}, fmt.Errorf("ошибка отправки сообщения: %w", err)
	}

	return sentMsg, nil
}

// MessageOption определяет опцию для настройки отправляемого сообщения
type MessageOption func(*tgbotapi.MessageConfig)

// WithReplyMarkup добавляет клавиатуру к сообщению
func WithReplyMarkup(markup interface{}) MessageOption {
	return func(msg *tgbotapi.MessageConfig) {
		msg.ReplyMarkup = markup
	}
}

// WithParseMode устанавливает режим парсинга для сообщения
func WithParseMode(parseMode string) MessageOption {
	return func(msg *tgbotapi.MessageConfig) {
		msg.ParseMode = parseMode
	}
}

// WithReplyToMessageID устанавливает ID сообщения, на которое отвечаем
func WithReplyToMessageID(messageID int) MessageOption {
	return func(msg *tgbotapi.MessageConfig) {
		msg.ReplyToMessageID = messageID
	}
}

// WithWebAppInfo добавляет ссылку на Mini App
func WithWebAppInfo() MessageOption {
	return func(msg *tgbotapi.MessageConfig) {
		// Создаем клавиатуру с кнопкой "Открыть Профиль"
		keyboard := tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Открыть Профиль"),
			),
		)
		keyboard.ResizeKeyboard = true
		msg.ReplyMarkup = keyboard
	}
}
