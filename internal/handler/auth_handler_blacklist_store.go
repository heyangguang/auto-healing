package handler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/database"
	"github.com/redis/go-redis/v9"
)

const authTokenBlacklistPrefix = "token_blacklist:"

var errAuthBlacklistStoreUnavailable = errors.New("token blacklist store is unavailable")

type authTokenBlacklistStore struct {
	client *redis.Client
	prefix string
}

func newAuthTokenBlacklistStore() *authTokenBlacklistStore {
	return &authTokenBlacklistStore{
		client: database.RedisClient,
		prefix: authTokenBlacklistPrefix,
	}
}

func (s *authTokenBlacklistStore) Add(ctx context.Context, jti string, exp time.Time) error {
	if s.client == nil {
		return errAuthBlacklistStoreUnavailable
	}
	ttl := time.Until(exp)
	if ttl <= 0 {
		return nil
	}
	if err := s.client.Set(ctx, s.prefix+jti, "1", ttl).Err(); err != nil {
		return fmt.Errorf("write token blacklist: %w", err)
	}
	return nil
}

func (s *authTokenBlacklistStore) Exists(ctx context.Context, jti string) (bool, error) {
	if s.client == nil {
		return true, errAuthBlacklistStoreUnavailable
	}
	result, err := s.client.Exists(ctx, s.prefix+jti).Result()
	if err != nil {
		return true, fmt.Errorf("read token blacklist: %w", err)
	}
	return result > 0, nil
}
