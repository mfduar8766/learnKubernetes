package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/mfduar8766/learnKubernetes/views"
	"github.com/redis/go-redis/v9"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func connectToMongo(ctx context.Context, logger *slog.Logger) (*mongo.Client, error) {
	defaultHost := "mongo-service:27017"

	user := os.Getenv("MONGO_INITDB_ROOT_USERNAME")
	pass := os.Getenv("MONGO_INITDB_ROOT_PASSWORD")
	host := os.Getenv("MONGO_HOST")

	if host == "" || user == "" || pass == "" {
		logger.Warn("Main::main()::Mongo env vars missing, using defaults", "host", defaultHost)
		if host == "" {
			host = defaultHost
		}
		if user == "" {
			user = "user"
		}
		if pass == "" {
			pass = "password"
		}
	}

	mongoDSN := fmt.Sprintf("mongodb://%s:%s@%s", user, pass, host)

	logger.Info("Main::main()::Attempting Mongo connection", "DSN", fmt.Sprintf("mongodb://%s:****@%s", user, host))

	clientOptions := options.Client().ApplyURI(mongoDSN)

	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, err
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}

	logger.Info("Main::main()::Connected to MongoDB")
	return client, nil
}

func connectToRedis(ctx context.Context, logger *slog.Logger) *redis.Client {
	var (
		redisHost     = "redis-service:6379"
		redisPassword = "password"
	)

	if host := os.Getenv("REDIS_URL"); len(host) == 0 {
		logger.Error("Main::connectToRedis()::No redis host configured. Using default host instead")
	}
	if password := os.Getenv("REDIS_PASSWORD"); len(password) == 0 {
		logger.Error("Main::connectToRedis()::No redis password configured. Using default password instead")
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
		log.Fatalf("Main::connectToRedis()::Failed to connect to Redis: %v", err)
	}
	logger.Info("Main::connectToRedis()::connected to redis", "host", redisHost)
	return redisClient
}

func main() {
	var (
		ctx         = context.Background()
		wg          sync.WaitGroup
		logger                    = slog.New(slog.NewJSONHandler(os.Stdout, nil))
		redisClient *redis.Client = nil
		mongoClient *mongo.Client = nil
		err         error         = nil
	)
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	redisOnce := sync.OnceValue(func() *redis.Client {
		return connectToRedis(ctx, logger)
	})
	redisClient = redisOnce()

	mongoOnce := sync.OnceValues(func() (*mongo.Client, error) {
		return connectToMongo(ctx, logger)
	})
	mongoClient, err = mongoOnce()
	if err != nil {
		return
	}

	srv := &http.Server{
		Addr:    ":3000",
		Handler: http.DefaultServeMux,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		views.Hello("Foo").Render(ctx, w)
	})

	wg.Go(func() {
		if err = srv.ListenAndServe(); err != nil && err == http.ErrServerClosed {
			logger.Error("Main::main()::serverClosed", "error", err.Error())
			cancel()
		}
	})

	logger.Info("Main::main()::Server is running on", "host", "127.0.0.1", "port", "3000")

	<-ctx.Done()
	logger.Info("Main::main()::received shutdown signal...")

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 5*time.Second)
	defer shutdownCancel()

	if err = srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Main::main()::Server forced shutdown", "error", err.Error())
	}

	logger.Info("Main::main()::Closing database connections...")

	if err = redisClient.Close(); err != nil {
		logger.Error("Main::main()::Failed to disconnect from redis", "error", err)
	}

	if err = mongoClient.Disconnect(shutdownCtx); err != nil {
		logger.Error("Main::main()::Failed to disconnect from mongo", "error", err)
	}

	logger.Info("Main::main()::App exited cleanly")
	wg.Wait()
}
