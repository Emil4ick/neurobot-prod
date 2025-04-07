package user

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/binary"
	"fmt"
	"sync"
)

// ShardedRepository представляет репозиторий с поддержкой шардирования данных
type ShardedRepository struct {
	shards    []*Repository
	numShards int
	mu        sync.RWMutex
}

// NewShardedRepository создает новый шардированный репозиторий
func NewShardedRepository(dbs []*sql.DB) *ShardedRepository {
	numShards := len(dbs)
	if numShards == 0 {
		panic("требуется хотя бы одна база данных для шардирования")
	}

	shards := make([]*Repository, numShards)
	for i, db := range dbs {
		shards[i] = NewRepository(db)
	}

	return &ShardedRepository{
		shards:    shards,
		numShards: numShards,
	}
}

// shardByTelegramID определяет шард по Telegram ID
func (r *ShardedRepository) shardByTelegramID(telegramID int64) int {
	// Используем md5 для равномерного распределения
	hash := md5.Sum([]byte(fmt.Sprintf("%d", telegramID)))
	return int(binary.BigEndian.Uint32(hash[:4]) % uint32(r.numShards))
}

// UpsertUser создает пользователя на соответствующем шарде
func (r *ShardedRepository) UpsertUser(ctx context.Context, telegramID int64, username, firstName, lastName, languageCode string, isBot bool) (*User, error) {
	shardIndex := r.shardByTelegramID(telegramID)

	r.mu.RLock()
	shard := r.shards[shardIndex]
	r.mu.RUnlock()

	return shard.UpsertUser(ctx, telegramID, username, firstName, lastName, languageCode, isBot)
}

// GetByTelegramID получает пользователя с соответствующего шарда
func (r *ShardedRepository) GetByTelegramID(ctx context.Context, telegramID int64) (*User, error) {
	shardIndex := r.shardByTelegramID(telegramID)

	r.mu.RLock()
	shard := r.shards[shardIndex]
	r.mu.RUnlock()

	return shard.GetByTelegramID(ctx, telegramID)
}

// CountUsers считает количество пользователей на всех шардах
func (r *ShardedRepository) CountUsers(ctx context.Context) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var totalCount int
	var wg sync.WaitGroup
	var mu sync.Mutex
	var countErr error

	for _, shard := range r.shards {
		wg.Add(1)

		go func(shard *Repository) {
			defer wg.Done()

			count, err := shard.CountUsers(ctx)
			if err != nil {
				mu.Lock()
				countErr = err
				mu.Unlock()
				return
			}

			mu.Lock()
			totalCount += count
			mu.Unlock()
		}(shard)
	}

	wg.Wait()

	if countErr != nil {
		return 0, countErr
	}

	return totalCount, nil
}
