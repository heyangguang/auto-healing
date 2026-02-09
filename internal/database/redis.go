package database

import (
	"context"
	"fmt"
	"time"

	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// RedisClient 全局 Redis 客户端
var RedisClient *redis.Client

// InitRedis 初始化 Redis 连接
func InitRedis(cfg *config.RedisConfig) error {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := RedisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis 连接失败: %w", err)
	}

	logger.Info("Redis 连接成功")
	return nil
}

// CloseRedis 关闭 Redis 连接
func CloseRedis() error {
	if RedisClient != nil {
		return RedisClient.Close()
	}
	return nil
}

// TokenBlacklistStore Token 黑名单存储 (实现 jwt.BlacklistStore 接口)
type TokenBlacklistStore struct {
	client *redis.Client
	prefix string
}

// NewTokenBlacklistStore 创建 Token 黑名单存储
func NewTokenBlacklistStore() *TokenBlacklistStore {
	return &TokenBlacklistStore{
		client: RedisClient,
		prefix: "token_blacklist:",
	}
}

// Add 添加 Token 到黑名单
func (s *TokenBlacklistStore) Add(ctx context.Context, jti string, exp time.Time) error {
	ttl := time.Until(exp)
	if ttl <= 0 {
		return nil // Token 已过期，无需加入黑名单
	}
	return s.client.Set(ctx, s.prefix+jti, "1", ttl).Err()
}

// Exists 检查 Token 是否在黑名单中
func (s *TokenBlacklistStore) Exists(ctx context.Context, jti string) bool {
	result, err := s.client.Exists(ctx, s.prefix+jti).Result()
	if err != nil {
		return false
	}
	return result > 0
}

// LogStreamPublisher 日志流发布器
type LogStreamPublisher struct {
	client *redis.Client
}

// NewLogStreamPublisher 创建日志流发布器
func NewLogStreamPublisher() *LogStreamPublisher {
	return &LogStreamPublisher{
		client: RedisClient,
	}
}

// Publish 发布日志到 Redis Stream
func (p *LogStreamPublisher) Publish(ctx context.Context, streamKey string, data map[string]interface{}) error {
	return p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: data,
	}).Err()
}

// Subscribe 订阅日志流
func (p *LogStreamPublisher) Subscribe(ctx context.Context, streamKey string, lastID string, handler func(id string, values map[string]interface{})) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			streams, err := p.client.XRead(ctx, &redis.XReadArgs{
				Streams: []string{streamKey, lastID},
				Block:   5 * time.Second,
				Count:   100,
			}).Result()

			if err != nil {
				if err == redis.Nil {
					continue
				}
				return err
			}

			for _, stream := range streams {
				for _, msg := range stream.Messages {
					lastID = msg.ID
					handler(msg.ID, msg.Values)
				}
			}
		}
	}
}
