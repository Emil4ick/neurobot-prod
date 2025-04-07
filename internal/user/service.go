package user

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Service предоставляет методы для работы с пользователями
type Service struct {
	repo  *Repository
	redis *redis.Client
	log   *zap.Logger

	// Настройки кэширования
	cacheTTL  time.Duration
	cacheKeys struct {
		userByTelegramID string // Шаблон для ключа кэша
	}
}

// NewService создает новый сервис пользователей
func NewService(repo *Repository, redis *redis.Client, log *zap.Logger) *Service {
	return &Service{
		repo:     repo,
		redis:    redis,
		log:      log.Named("user_service"),
		cacheTTL: 1 * time.Hour, // Кэшируем на 1 час
		cacheKeys: struct {
			userByTelegramID string
		}{
			userByTelegramID: "user:telegram:%d", // Шаблон для ключа кэша
		},
	}
}

// EnsureUserExists проверяет, существует ли пользователь, создает или обновляет его
// Оптимизирован для высокой нагрузки с использованием кэширования и атомарных операций
func (s *Service) EnsureUserExists(ctx context.Context, telegramID int64, username, firstName, lastName, languageCode string, isBot bool) (*UserDTO, error) {
	// Формируем ключ кэша
	cacheKey := fmt.Sprintf(s.cacheKeys.userByTelegramID, telegramID)

	// Проверяем кэш сначала - это самая быстрая операция
	cachedUserDTO, err := s.getUserFromCache(ctx, cacheKey)
	if err == nil && cachedUserDTO != nil {
		return cachedUserDTO, nil
	}

	// Если пользователя нет в кэше, создаем или обновляем его в БД
	// Используем контекст с таймаутом для предотвращения длительных блокировок
	dbCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// Замеряем время выполнения DB операции для анализа производительности
	startTime := time.Now()

	user, err := s.repo.UpsertUser(dbCtx, telegramID, username, firstName, lastName, languageCode, isBot)
	if err != nil {
		s.log.Error("Ошибка при создании/обновлении пользователя",
			zap.Int64("telegram_id", telegramID),
			zap.Error(err),
			zap.Duration("duration", time.Since(startTime)))
		return nil, fmt.Errorf("ошибка при создании/обновлении пользователя: %w", err)
	}

	// Логируем время выполнения для анализа
	s.log.Debug("Пользователь создан/обновлен в БД",
		zap.Int64("user_id", user.ID),
		zap.Int64("telegram_id", telegramID),
		zap.Duration("duration", time.Since(startTime)))

	// Преобразуем к DTO для безопасной сериализации
	userDTO := user.ToUserDTO()

	// Асинхронно обновляем кэш, не блокируя основной поток
	go func() {
		// Создаем новый контекст для асинхронной операции
		cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cacheCancel()

		if err := s.cacheUser(cacheCtx, cacheKey, &userDTO); err != nil {
			s.log.Warn("Не удалось кэшировать пользователя",
				zap.Int64("telegram_id", telegramID),
				zap.Error(err))
		}
	}()

	return &userDTO, nil
}

// getUserFromCache получает пользователя из кэша Redis
func (s *Service) getUserFromCache(ctx context.Context, key string) (*UserDTO, error) {
	startTime := time.Now()

	data, err := s.redis.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			// Ключ не найден в кэше, это нормально
			return nil, nil
		}
		// Реальная ошибка Redis
		s.log.Warn("Ошибка получения данных из Redis",
			zap.String("key", key),
			zap.Error(err),
			zap.Duration("duration", time.Since(startTime)))
		return nil, err
	}

	var userDTO UserDTO
	if err := json.Unmarshal(data, &userDTO); err != nil {
		s.log.Warn("Ошибка десериализации данных пользователя из кэша",
			zap.String("key", key),
			zap.Error(err),
			zap.Duration("duration", time.Since(startTime)))
		return nil, err
	}

	s.log.Debug("Пользователь получен из кэша",
		zap.String("key", key),
		zap.Duration("duration", time.Since(startTime)))

	return &userDTO, nil
}

// cacheUser сохраняет пользователя в кэше Redis
func (s *Service) cacheUser(ctx context.Context, key string, user *UserDTO) error {
	startTime := time.Now()

	data, err := json.Marshal(user)
	if err != nil {
		return err
	}

	// Устанавливаем значение с TTL
	if err := s.redis.Set(ctx, key, data, s.cacheTTL).Err(); err != nil {
		return err
	}

	s.log.Debug("Пользователь сохранен в кэше",
		zap.String("key", key),
		zap.Duration("duration", time.Since(startTime)))

	return nil
}

// Дополнительные методы...

// GetUserByTelegramID получает пользователя по Telegram ID с использованием кэша
func (s *Service) GetUserByTelegramID(ctx context.Context, telegramID int64) (*UserDTO, error) {
	// Проверяем кэш
	cacheKey := fmt.Sprintf(s.cacheKeys.userByTelegramID, telegramID)
	cachedUserDTO, err := s.getUserFromCache(ctx, cacheKey)
	if err == nil && cachedUserDTO != nil {
		return cachedUserDTO, nil
	}

	// Если нет в кэше, читаем из БД
	user, err := s.repo.GetByTelegramID(ctx, telegramID)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, fmt.Errorf("пользователь не найден")
	}

	// Преобразуем к DTO
	userDTO := user.ToUserDTO()

	// Асинхронно обновляем кэш
	go func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		if err := s.cacheUser(cacheCtx, cacheKey, &userDTO); err != nil {
			s.log.Warn("Не удалось кэшировать пользователя",
				zap.Int64("telegram_id", telegramID),
				zap.Error(err))
		}
	}()

	return &userDTO, nil
}

// InvalidateUserCache удаляет пользователя из кэша
func (s *Service) InvalidateUserCache(ctx context.Context, telegramID int64) {
	cacheKey := fmt.Sprintf(s.cacheKeys.userByTelegramID, telegramID)
	if err := s.redis.Del(ctx, cacheKey).Err(); err != nil {
		s.log.Warn("Ошибка при удалении пользователя из кэша",
			zap.Int64("telegram_id", telegramID),
			zap.Error(err))
	}
}

// GetRegisteredUserCount возвращает количество зарегистрированных пользователей
func (s *Service) GetRegisteredUserCount(ctx context.Context) (int, error) {
	// Проверяем кэш сначала
	const countCacheKey = "user:count"

	// Пытаемся получить счетчик из кэша
	cachedCount, err := s.redis.Get(ctx, countCacheKey).Int()
	if err == nil {
		return cachedCount, nil
	}

	// Если в кэше нет или ошибка, считаем из БД
	count, err := s.repo.CountUsers(ctx)
	if err != nil {
		return 0, err
	}

	// Кэшируем результат на 10 минут
	s.redis.Set(ctx, countCacheKey, strconv.Itoa(count), 10*time.Minute)

	return count, nil
}
