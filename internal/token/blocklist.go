package token

import (
	"context"
	"time"
	"todo_server/internal/cache"
)

const blockedPrefix = "blocklist:"

type Blocklist struct {
	cache *cache.RedisCache
}

func NewBlocklist(cache *cache.RedisCache) *Blocklist {
	return &Blocklist{cache: cache}
}

func (b *Blocklist) Block(ctx context.Context, tokenStr string, ttl time.Duration) error {
	key := blockedPrefix + tokenStr
	return b.cache.Set(ctx, key, "1")
}

func (b *Blocklist) IsBlocked(ctx context.Context, tokenStr string) bool {
	key := blockedPrefix + tokenStr
	return b.cache.Exists(ctx, key)
}