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
	"time"

	"github.com/eclipse/paho.golang/paho"
	"github.com/mfduar8766/learnKubernetes/handlers"
	"github.com/mfduar8766/learnKubernetes/lib/httpServer"
	"github.com/mfduar8766/learnKubernetes/lib/logger"
	"github.com/mfduar8766/learnKubernetes/lib/transport"
	"github.com/mfduar8766/learnKubernetes/lib/types"
	"github.com/mfduar8766/learnKubernetes/lib/utils"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type AppDeps struct {
	server  httpServer.IServer
	redis   *redis.Client
	handler handlers.IRequestHandler
	broker  transport.ITransport
	log     logger.ILogger
	mongo   *mongo.Client
}

func NewAppDeps(ctx context.Context) func() *AppDeps {
	return sync.OnceValue(func() *AppDeps {
		log := logger.NewLogger(types.APP_USERS_SERVICE)
		// redisClient := redisDb.ConnectToRedis(ctx, log)
		// mongoClient, err := mongoDb.ConnectToMongo(ctx, log)
		// if err != nil {
		// 	panic(err)
		// }
		// broker := rmq.NewRmq(ctx, log, types.APP_USERS_SERVICE)
		broker := transport.New(log)
		err := broker.Connect(ctx, types.APP_USERS_SERVICE, false)
		if err != nil {
			panic(err)
		}
		srv := httpServer.New(ctx, log, utils.GetHostPort(types.APP_USERS_SERVICE))
		handler := handlers.New(srv.GetCtx(), broker, log)
		// handler.Subscribe(broker.BuildTopic(rmq.USERS_EX, rmq.USERS_QUEUE, rmq.Request), broker.BuildTopic(rmq.POSTS_EX, rmq.POSTS_QUEUE, rmq.Events))
		handler.Subscribe(broker.BuildTopic(transport.TOPIC_TYPE_REQUEST, "users"), broker.BuildTopic(transport.TOPIC_TYPE_EVENT, "users")) //handler.Subscribe(broker.BuildTopic(rmq.USERS_EX, rmq.USERS
		appDeps := AppDeps{
			server: srv,
			// redis:   redisClient,
			handler: handler,
			broker:  broker,
			log:     log,
			// mongo:   mongoClient,
		}
		return &appDeps
	})
}

func main() {
	var (
		wg    sync.WaitGroup
		ctx            = context.Background()
		app   *AppDeps = NewAppDeps(ctx)()
		users *Users   = NewUsers(app.broker, app.log)
	)
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	defer func() {
		app.handler.Unsubscribe(app.broker.BuildTopic(transport.TOPIC_TYPE_REQUEST, "users"), app.broker.BuildTopic(transport.TOPIC_TYPE_EVENT, "users"))
		err := app.broker.Close()
		if err != nil {
			app.log.LogErrorf("AppGateWay::Start()::Failed to disconnect from broker: %+v", err)
		}
		cancel()
		app.log.Close()
	}()

	app.server.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		if app.redis == nil || app.broker == nil || app.mongo == nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "%+v", errors.New("failed to connect to dbs..."))
			return
		}
		if !strings.Contains(r.Header.Get(types.HEADER_USER_AGENT), "kube") {
			app.log.LogInfo(&logger.LoggerPayload{
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
	topic := app.broker.BuildTopic(transport.TOPIC_TYPE_REQUEST, "users")
	app.broker.RegisterHandler(topic, func(p *paho.Publish) {
		app.log.LogInfof("Processing: %+v", map[string]any{
			"topic":         p.Topic,
			"payload":       string(p.Payload),
			"responseTopic": p.Properties.ResponseTopic,
		})
		response, err := users.GetUsers()
		utils.HandleError(err, "main()", "Error getting users...", app.log)
		if err != nil {
			return
		}
		messageID, err := app.broker.Publish(ctx, p.Properties.ResponseTopic, response, nil)
		if err != nil {
			app.log.LogErrorf("Failed to publish response for messageID: %s on topic %s: %+v", messageID, topic, err)
		}
	})

	wg.Go(func() {
		if err := app.server.ListenAndServe(); err != nil && err == http.ErrServerClosed {
			app.log.LogErrorf("Main::main()::ListenAndServe()::%+v", err.Error())
			cancel()
		}
	})

	app.log.LogInfo(&logger.LoggerPayload{
		Message: "Main::main()::Server is running on",
		Value: map[string]string{
			"host": "127.0.0.1",
			"port": "3001",
		},
	})

	<-ctx.Done()
	shutDownCtx, shutdownCancel := context.WithTimeout(ctx, 5*time.Second)
	defer shutdownCancel()

	if err := app.redis.Close(); err != nil {
		app.log.LogErrorf("Main::main()::Failed to disconnect from redis: %+v", err.Error())
	}

	if err := app.mongo.Disconnect(shutDownCtx); err != nil {
		app.log.LogErrorf("Main::main()::Failed to disconnect from mongo: %+v", err.Error())
	}

	// if err := app.broker.Close(); err != nil {
	// 	app.log.LogErrorf("Main::main()::Failed to disconnect from broker: %+v", err)
	// }

	app.server.Shutdown(shutDownCtx)

	app.log.LogInfof("Main::main()::App exited cleanly")
}
