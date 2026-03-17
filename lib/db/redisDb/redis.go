package redisDb

import (
	"context"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

func ConnectToRedis(ctx context.Context, logger *slog.Logger) *redis.Client {
	var (
		redisHost     = "redis-service:6379"
		redisPassword = "password"
	)

	if host := os.Getenv("REDIS_URL"); len(host) == 0 {
		logger.Error("Redis::connectToRedis()::No redis host configured. Using default host instead")
	}
	if password := os.Getenv("REDIS_PASSWORD"); len(password) == 0 {
		logger.Error("Redis::connectToRedis()::No redis password configured. Using default password instead")
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisHost,
		Password: redisPassword,
		DB:       0,
		// Retry configuration
		MaxRetries:      5,                      // Maximum number of retries
		MinRetryBackoff: 8 * time.Millisecond,   // Minimum backoff between retries
		MaxRetryBackoff: 512 * time.Millisecond, // Maximum backoff between retries
	})
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Redis::connectToRedis()::Failed to connect to Redis: %v", err)
	}
	logger.Info("Redis::connectToRedis()::connected to redis", "host", redisHost)
	return redisClient
}
