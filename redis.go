package main

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// RedisManager handles Redis connections and subscriptions
type RedisManager struct {
	client *redis.Client
	logger zerolog.Logger
}

// NewRedisManager creates a new Redis manager
func NewRedisManager(config Config) (*RedisManager, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         config.RedisAddr,
		Password:     config.RedisPassword,
		DB:           config.RedisDB,
		PoolSize:     config.RedisPoolSize,
		MinIdleConns: config.RedisPoolSize / 4,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		PoolTimeout:  6 * time.Second,
	})

	// Verify Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisManager{
		client: client,
		logger: log.With().Str("component", "redisManager").Logger(),
	}, nil
}

// Publish publishes a message to Redis
func (rm *RedisManager) Publish(ctx context.Context, topic string, message []byte) error {
	return rm.client.Publish(ctx, topic, message).Err()
}

// Subscribe subscribes to a Redis topic
func (rm *RedisManager) Subscribe(ctx context.Context, topic string) *redis.PubSub {
	return rm.client.Subscribe(ctx, topic)
}

// Close closes the Redis client
func (rm *RedisManager) Close() error {
	return rm.client.Close()
}
