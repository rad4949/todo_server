package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisCache(client *redis.Client, ttl time.Duration) *RedisCache {
	return &RedisCache{
		client: client,
		ttl:    ttl,
	}
}

func (c *RedisCache) Set(ctx context.Context, key, value string) error {
	return c.client.Set(ctx, key, value, c.ttl).Err()
}

func (c *RedisCache) Get(ctx context.Context, key string) (string, bool) {
	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		return "", false
	}
	return val, true
}

func (c *RedisCache) Exists(ctx context.Context, key string) bool {
	n, err := c.client.Exists(ctx, key).Result()
	return err == nil && n > 0
}

func (c *RedisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

func (c *RedisCache) Increment(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	attempts, err := c.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}

	if attempts == 1 {
		c.client.Expire(ctx, key, ttl)
	}

	return attempts, nil
}

func (c *RedisCache) SetWithTTL(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}