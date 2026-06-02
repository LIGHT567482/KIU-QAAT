package store

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const jtiPrefix = "jti:blacklist:"

type TokenStore struct {
	rdb *redis.Client
}

func NewTokenStore(rdb *redis.Client) *TokenStore {
	return &TokenStore{rdb: rdb}
}

// Blacklist adds a jti to the Redis blacklist with a TTL equal to the
// remaining token lifetime.  Called on logout and token refresh.
func (s *TokenStore) Blacklist(ctx context.Context, jti string, ttl time.Duration) error {
	key := jtiPrefix + jti
	return s.rdb.Set(ctx, key, "1", ttl).Err()
}

// IsBlacklisted returns true if the jti is in the Redis blacklist.
func (s *TokenStore) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	key := jtiPrefix + jti
	res, err := s.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("redis exists: %w", err)
	}
	return res > 0, nil
}
