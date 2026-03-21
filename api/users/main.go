package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/mfduar8766/learnKubernetes/handlers"
	"github.com/mfduar8766/learnKubernetes/lib/httpServer"
	"github.com/mfduar8766/learnKubernetes/lib/logger"
	"github.com/mfduar8766/learnKubernetes/lib/models"
	"github.com/mfduar8766/learnKubernetes/lib/rmq"
	"github.com/mfduar8766/learnKubernetes/lib/types"
	"github.com/mfduar8766/learnKubernetes/lib/utils"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type AppDeps struct {
	Server  httpServer.IServer
	Redis   *redis.Client
	Handler *handlers.RequestHandler
	broker  rmq.IRabbitMq
	Log     logger.ILogger
	Mongo   *mongo.Client
}

func NewAppDeps(ctx context.Context) func() *AppDeps {
	return sync.OnceValue(func() *AppDeps {
		log := logger.NewLogger(types.APP_GATE_WAY)
		// redisClient := redisDb.ConnectToRedis(ctx, log)
		// mongoClient, err := mongoDb.ConnectToMongo(ctx, log)
		// if err != nil {
		// 	panic(err)
		// }
		broker := rmq.NewRmq(ctx, log, types.APP_GATE_WAY)
		srv := httpServer.NewServer(ctx, log, 3001)
		handler := handlers.NewHandler(srv.GetCtx(), broker, log)
		handler.Subscribe(broker.BuildTopic(rmq.USERS_EX, rmq.USERS_QUEUE, rmq.Request), broker.BuildTopic(rmq.POSTS_EX, rmq.POSTS_QUEUE, rmq.Events))

		appDeps := AppDeps{
			Server: srv,
			// Redis:   redisClient,
			Handler: handler,
			broker:  broker,
			Log:     log,
			// Mongo:   mongoClient,
		}
		return &appDeps
	})
}

func main() {
	var (
		wg    sync.WaitGroup
		ctx            = context.Background()
		app   *AppDeps = NewAppDeps(ctx)()
		users *Users   = NewUsers(app.broker, app.Log)
	)
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	defer func() {
		app.broker.Close()
		cancel()
		app.Log.Close()
	}()

	app.Server.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		if app.Redis == nil || app.broker == nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "%+v", errors.New("failed to connect to dbs..."))
			return
		}
		if !strings.Contains(r.Header.Get(types.USER_AGENT), "kube") {
			app.Log.LogInfo(&logger.LoggerPayload{
				Message: "Main::main()::received readiness check on",
				Value: map[string]any{
					"host":    r.Host,
					"headers": r.Header,
					"method":  r.Method,
				},
			})
		}
		w.WriteHeader(http.StatusOK)
	})

	app.broker.Listen(app.broker.BuildTopic(rmq.USERS_EX, rmq.USERS_QUEUE, rmq.Request), func(req *models.MessagePayload) ([]byte, error) {
		app.Log.LogInfof("Processing event: %s on topic: %+v", req.Event, req.Topic)
		response, err := users.GetUsers()
		utils.HandleError(err, "main()", "Error getting users...", app.Log)
		return response, nil
	})

	wg.Go(func() {
		if err := app.Server.ListenAndServe(); err != nil && err == http.ErrServerClosed {
			app.Log.LogErrorf("Main::main()::ListenAndServe()::%+v", err.Error())
			cancel()
		}
	})

	app.Log.LogInfo(&logger.LoggerPayload{
		Message: "Main::main()::Server is running on",
		Value: map[string]string{
			"host": "127.0.0.1",
			"port": "3001",
		},
	})

	<-ctx.Done()
	// shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 5*time.Second)
	// defer shutdownCancel()

	// if err := app.Redis.Close(); err != nil {
	// 	app.Log.LogErrorf("Main::main()::Failed to disconnect from redis: %+v", err.Error())
	// }

	// if err := app.Mongo.Disconnect(shutdownCtx); err != nil {
	// 	app.Log.LogErrorf("Main::main()::Failed to disconnect from mongo: %+v", err.Error())
	// }

	app.Log.LogInfof("Main::main()::App exited cleanly")
}
