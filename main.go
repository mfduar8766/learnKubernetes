package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mfduar8766/learnKubernetes/lib"
	"github.com/mfduar8766/learnKubernetes/lib/db/mongoDb"
	"github.com/mfduar8766/learnKubernetes/lib/db/redisDb"
	"github.com/mfduar8766/learnKubernetes/lib/httpServer"
	"github.com/mfduar8766/learnKubernetes/lib/models"
	"github.com/mfduar8766/learnKubernetes/views"
	"github.com/redis/go-redis/v9"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

func auth(ctx context.Context, logger *slog.Logger, headers http.Header) error {
	logger.Info("Main::main()::received auth request...")
	if headers == nil {
		return fmt.Errorf("args cannot be nil")
	}
	tokenHeader := headers.Get(lib.HEADER_TOKEN)
	if len(tokenHeader) == 0 {
		return fmt.Errorf("token is empty")
	} else {
		if tokenHeader == "SOME_TOKEN" {
			return nil
		}
		return fmt.Errorf("incorrect token...")
	}
}

func Handler(ctx context.Context, logger *slog.Logger, handler httpServer.HttpHandler, middleWare ...httpServer.MiddleWareOptions) httpServer.HttpHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		if len(middleWare) > 0 {
			for _, mid := range middleWare {
				if err := mid(ctx, logger, r.Header); err != nil {
					logger.Error("Main::main()::received error", "error", err.Error())
					w.WriteHeader(http.StatusUnauthorized)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}
		logger.Info("Calling handler...")
		handler(w, r)
	}
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
		return redisDb.ConnectToRedis(ctx, logger)
	})
	redisClient = redisOnce()

	mongoOnce := sync.OnceValues(func() (*mongo.Client, error) {
		return mongoDb.ConnectToMongo(ctx, logger)
	})
	mongoClient, err = mongoOnce()
	if err != nil {
		return
	}

	srv := &http.Server{
		Addr:    ":3000",
		Handler: http.DefaultServeMux,
	}

	http.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		views.Hello("Foo").Render(ctx, w)
	})

	// Readiness Check
	http.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		if redisClient == nil || mongoClient == nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "%+v", errors.New("failed to connect to dbs..."))
			return
		}
		if !strings.Contains(r.Header.Get(lib.USER_AGENT), "kube") {
			logger.Info("Main::main()::received readiness check", "host", r.Host, "headers", r.Header, "method", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})

	// Liveliness Check
	http.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get(lib.USER_AGENT), "kube") {
			logger.Info("Main::main()::received health check", "host", r.Host, "headers", r.Header, "method", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("POST /api/post", func(w http.ResponseWriter, r *http.Request) {
		logger.Info("Main::main()::received request", "host", r.Host, "headers", r.Header, "method", r.Method)

		bytes, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "%+v", err.Error())
		}
		defer r.Body.Close()

		logger.Info("Main::main()::received data", "body", string(bytes))
		w.WriteHeader(http.StatusCreated)
	})

	http.HandleFunc("GET /api/posts", Handler(ctx, logger, func(w http.ResponseWriter, r *http.Request) {
		logger.Info("Main::main()::received GET for posts")
		// 1. Set the content type FIRST
		w.Header().Set(lib.CONTENT_TYPE, lib.APPLICATION_JSON)

		postBytes, err := os.ReadFile("posts.json")
		if err != nil {
			http.Error(w, "File not found", http.StatusInternalServerError)
			return
		}

		var posts []models.Posts
		err = json.Unmarshal(postBytes, &posts)
		if err != nil {
			http.Error(w, "Failed to parse JSON", http.StatusInternalServerError)
			return
		}

		// 2. Set the status code SECOND
		w.WriteHeader(http.StatusOK)

		// 3. Write the body LAST
		err = json.NewEncoder(w).Encode(posts)
		if err != nil {
			log.Printf("Failed to encode JSON: %v", err)
		}
	}, auth))

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

	// var req models.Request[models.User] = models.Request[models.User]{
	// 	ApiName: "getUsers",
	// 	Body: models.User{},
	// }

	// s := lib.NewServer(ctx, logger, ":3000")
	// s.SetOptions(s.SetRoutes(lib.Routes{}))
	// s.ListenAndServe()
}
