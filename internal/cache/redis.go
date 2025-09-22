package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/oggyb/muzz-exercise/internal/config"
	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	Client *redis.Client
}

// NewRedisCache initializes Redis client from config.
// Only Addr is mandatory, Password/DB are optional.
func NewRedisCache(cfg *config.Config) *RedisCache {
	opts := &redis.Options{
		Addr: cfg.Redis.Addr,
	}
	if cfg.Redis.Password != "" {
		opts.Password = cfg.Redis.Password
	}
	if cfg.Redis.DB != 0 {
		opts.DB = cfg.Redis.DB
	}
	return &RedisCache{Client: redis.NewClient(opts)}
}

func (c *RedisCache) Ping(ctx context.Context) error {
	return c.Client.Ping(ctx).Err()
}

func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return c.Client.Set(ctx, key, value, ttl).Err()
}

func (c *RedisCache) Get(ctx context.Context, key string) (string, error) {
	return c.Client.Get(ctx, key).Result()
}

func (c *RedisCache) Del(ctx context.Context, key string) error {
	return c.Client.Del(ctx, key).Err()
}

func (c *RedisCache) Incr(ctx context.Context, key string) (int64, error) {
	return c.Client.Incr(ctx, key).Result()
}

func (c *RedisCache) Decr(ctx context.Context, key string) (int64, error) {
	return c.Client.Decr(ctx, key).Result()
}

// KeyForLikeCount generates Redis key for a user's like count
func (c *RedisCache) KeyForLikeCount(userID uint64) string {
	return fmt.Sprintf("likes:count:%d", userID)
}

func (c *RedisCache) UpdateLikeCount(ctx context.Context, userID uint64, count int64) error {
	key := fmt.Sprintf("likes:count:%d", userID)
	// Always refresh TTL when updating
	return c.Client.Set(ctx, key, count, time.Hour).Err()
}

func (c *RedisCache) GetLikeCount(ctx context.Context, userID uint64) (int64, error) {
	key := fmt.Sprintf("likes:count:%d", userID)
	val, err := c.Client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return 0, nil // cache miss
	} else if err != nil {
		return 0, err
	}
	// refresh TTL on access
	_ = c.Client.Expire(ctx, key, time.Hour).Err()
	return strconv.ParseInt(val, 10, 64)
}
