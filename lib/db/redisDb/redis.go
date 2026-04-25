package redisDb

import (
	"context"
	"strings"
	"time"

	"github.com/mfduar8766/learnKubernetes/lib/logger"
	"github.com/mfduar8766/learnKubernetes/lib/types"
	"github.com/mfduar8766/learnKubernetes/lib/utils"
	"github.com/redis/go-redis/v9"
)

func ConnectToRedis(ctx context.Context, log *logger.Logger) *redis.Client {
	connectionString, err := utils.CreateDbConnectionString(types.DB_REDIS, log)
	if err != nil {
		panic(err)
	}
	redisHost := connectionString[:strings.LastIndex(connectionString, "_")]
	redisPassword := connectionString[strings.LastIndex(connectionString, "_")+1:]
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisHost,
		Password: redisPassword,
		DB:       0,
		// Retry configuration
		MaxRetries:      5,                      // Maximum number of retries
		MinRetryBackoff: 8 * time.Millisecond,   // Minimum backoff between retries
		MaxRetryBackoff: 512 * time.Millisecond, // Maximum backoff between retries
	})
	_, err = redisClient.Ping(ctx).Result()
	if err != nil {
		log.LogErrorf("Redis::connectToRedis()::Failed to connect to Redis: %v", err.Error())
	}
	log.LogInfof("Redis::connectToRedis()::connected to redis on: %s", redisHost)
	return redisClient
}
