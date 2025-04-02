package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time" // Добавили для LeaseDuration, если понадобится

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"

	"neurobot-prod/internal/config" // !!! Указываем правильный модуль !!!
)

// Publisher отвечает за публикацию сообщений в NATS.
type Publisher struct {
	nc  *nats.Conn
	log *zap.Logger
	cfg config.NATSConfig
}

// NewPublisher создает новый экземпляр Publisher и подключается к NATS.
func NewPublisher(cfg config.NATSConfig, log *zap.Logger) (*Publisher, error) {
	logger := log.Named("nats_publisher")

	opts := []nats.Option{
		nats.Name("Neurobot Publisher (Webhook)"), // Имя клиента
		nats.Timeout(cfg.GetTimeout()),
		nats.ReconnectWait(cfg.GetReconnectWait()),
		nats.MaxReconnects(cfg.MaxReconnects),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			logArg := zap.Skip()
			if err != nil {
				logArg = zap.Error(err)
			}
			logger.Warn("NATS отключен", logArg)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info("NATS переподключен", zap.String("url", nc.ConnectedUrl()))
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			logger.Warn("NATS соединение закрыто")
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			logger.Error("NATS асинхронная ошибка", zap.Stringp("subject", safeGetSubject(sub)), zap.Error(err))
		}),
	}

	// Подключаемся к NATS
	nc, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		logger.Error("Ошибка подключения к NATS", zap.String("url", cfg.URL), zap.Error(err))
		return nil, fmt.Errorf("ошибка подключения к NATS %s: %w", cfg.URL, err)
	}
	logger.Info("Успешно подключен к NATS", zap.String("url", cfg.URL))

	return &Publisher{nc: nc, log: logger, cfg: cfg}, nil
}

// Publish сериализует данные в JSON и публикует в NATS.
func (p *Publisher) Publish(ctx context.Context, subject string, data interface{}) error {
	if p.nc == nil || !p.nc.IsConnected() { /* ... */
	}
	jsonData, err := json.Marshal(data)
	if err != nil { /* ... */
	}

	// Просто публикуем, без pubCtx
	err = p.nc.Publish(subject, jsonData) // Используем простой Publish

	if err != nil { /* ... */
	}
	p.log.Debug("Сообщение успешно опубликовано в NATS", zap.String("subject", subject), zap.Int("data_size", len(jsonData)))
	return nil
}

// Close корректно закрывает соединение с NATS.
func (p *Publisher) Close() {
	if p.nc != nil && !p.nc.IsClosed() {
		p.log.Info("Начинаем Drain NATS соединения Publisher...")

		// Создаем канал для сигнализации о завершении Drain
		done := make(chan bool)
		go func() {
			// Drain блокирует до завершения отправки буфера
			if err := p.nc.Drain(); err != nil {
				p.log.Error("Ошибка во время Drain NATS соединения", zap.Error(err))
			} else {
				p.log.Info("Drain NATS соединения Publisher завершен.")
			}
			close(done) // Сигнализируем, что Drain (или ошибка) завершился
		}()

		// Ожидаем завершения Drain ИЛИ таймаута
		select {
		case <-done:
			// Drain завершился (или произошла ошибка)
			p.log.Debug("Горутина Drain завершила работу.")
		case <-time.After(10 * time.Second): // Таймаут ожидания Drain
			p.log.Warn("Таймаут ожидания Drain NATS соединения (10 секунд). Соединение будет закрыто принудительно.")
		}

		p.nc.Close() // Закрываем соединение в любом случае
		p.log.Info("Соединение NATS Publisher закрыто.")
	}
}

// safeGetSubject безопасно получает имя темы из подписки
func safeGetSubject(sub *nats.Subscription) *string {
	if sub != nil {
		return &sub.Subject
	}
	return nil
}
