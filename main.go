package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mfduar8766/learnKubernetes/handlers"
	"github.com/mfduar8766/learnKubernetes/lib/db/redisDb"
	"github.com/mfduar8766/learnKubernetes/lib/httpServer"
	"github.com/mfduar8766/learnKubernetes/lib/logger"
	"github.com/mfduar8766/learnKubernetes/lib/rmq"
	"github.com/mfduar8766/learnKubernetes/lib/types"
	"github.com/mfduar8766/learnKubernetes/views"
	"github.com/redis/go-redis/v9"
)

type AppDeps struct {
	Server      *httpServer.Server
	Redis       *redis.Client
	Handler     *handlers.RequestHandler
	Broker      *rmq.RMQ
	Log         *logger.Logger
	TailWindCmd *exec.Cmd
}

func NewAppDeps(ctx context.Context) func() *AppDeps {
	return sync.OnceValue(func() *AppDeps {
		log := logger.NewLogger(types.APP_GATE_WAY)
		redisClient := redisDb.ConnectToRedis(ctx, log)
		broker := rmq.NewRmq(ctx, log, types.APP_GATE_WAY)
		srv := httpServer.NewServer(ctx, log, 3000)
		handler := handlers.NewHandler(srv.GetCtx(), broker, log)
		handler.Subscribe(broker.BuildTopic(rmq.USERS_EX, rmq.USERS_QUEUE, rmq.Request), broker.BuildTopic(rmq.POSTS_EX, rmq.POSTS_QUEUE, rmq.Events))
		appDeps := AppDeps{
			Server:  srv,
			Redis:   redisClient,
			Handler: handler,
			Broker:  broker,
			Log:     log,
		}
		appDeps.setUpTailWind()
		appDeps.launchTailWind()
		return &appDeps
	})
}

func (a *AppDeps) setUpTailWind() {
	var fileName = "tailwindcss"
	if _, err := os.Stat(fileName); err != nil {
		a.Log.LogInfof("Main::setUpTailWind()::%s file does not exist. Downloading latest build...", fileName)
		res, err := http.Get("https://github.com/tailwindlabs/tailwindcss/releases/download/v4.2.2/tailwindcss-linux-x64")
		if err != nil {
			panic(err)
		}
		if res.StatusCode == http.StatusOK {
			file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
			if err != nil {
				panic(err)
			}
			defer file.Close()
			io.Copy(file, res.Body)
			if err != nil {
				panic(err)
			}
			err = os.Chmod(fileName, 0755)
			if err != nil {
				panic(err)
			}
		}
	} else {
		err = os.Chmod(fileName, 0755)
		if err != nil {
			panic(err)
		}
	}
}

func (a *AppDeps) launchTailWind() {
	env := os.Getenv(types.CURRENT_ENV)

	args := []string{
		"-i", "./public/css/index.css",
		"-o", "./public/css/style.css",
		"--content", "./views/**/*.templ",
	}

	// ALWAYS do an initial build first, regardless of Env
	a.Log.LogInfof("⏳ Performing initial Tailwind build...")
	initialCmd := exec.Command("./tailwindcss", args...)
	if out, err := initialCmd.CombinedOutput(); err != nil {
		a.Log.LogErrorf("❌ Initial Tailwind build failed: %v\n%s", err, string(out))
	} else {
		a.Log.LogInfof("✅ Initial CSS generated.")
	}

	if env == types.PROD_ENV {
		return
	}

	// Now start the watcher for Dev
	watchArgs := append(args, "--watch")
	a.TailWindCmd = exec.Command("./tailwindcss", watchArgs...)
	a.TailWindCmd.Stdout = os.Stdout
	a.TailWindCmd.Stderr = os.Stderr

	if err := a.TailWindCmd.Start(); err != nil {
		a.Log.LogErrorf("❌ Failed to start Tailwind watcher: %v", err)
		return
	}
	go func() {
		err := a.TailWindCmd.Wait()
		if err != nil {
			a.Log.LogErrorf("Tailwind watcher exited: %v", err)
		}
	}()
}

func main() {
	var (
		ctx = context.Background()
		wg  sync.WaitGroup
		err error    = nil
		app *AppDeps = NewAppDeps(ctx)()
	)
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	defer func() {
		if app.TailWindCmd != nil && app.TailWindCmd.Process != nil {
			app.Log.LogInfof("Killing Tailwind watcher...")
			app.TailWindCmd.Process.Kill()
		}
		app.Broker.Connection.Close()
		app.Broker.Channel.Close()
		app.Broker.ClearSubscriptions()
		cancel()
		app.Log.Close()
	}()

	app.Server.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		if app.Redis == nil || app.Broker == nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "%+v", errors.New("failed to connect to dbs..."))
			return
		}

		if _, err := os.Stat("./public/css/style.css"); os.IsNotExist(err) {
			app.Log.LogErrorf("Main::main()::tailWind is not ready yet err: %s", err.Error())
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "CSS build in progress...")
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

	fileServer := http.FileServer(http.Dir("public"))
	// IN LATEST Go WE CANNOT USE /public/*
	app.Server.Get("/public/{file...}", func(w http.ResponseWriter, r *http.Request) {
		app.Log.LogInfof("Serving static file: %s", r.URL.Path)

		// If the path doesn't start with /public (because router stripped it), serve directly.
		// Otherwise, strip the prefix.
		if !strings.HasPrefix(r.URL.Path, "/public/") {
			fileServer.ServeHTTP(w, r)
			return
		}

		http.StripPrefix("/public/", fileServer).ServeHTTP(w, r)
	})

	app.Server.Get(types.API_ENDPOINT, func(w http.ResponseWriter, r *http.Request) {
		app.Handler.ProcessRequests(w, r)
	}, httpServer.RequestValidation, httpServer.Auth)

	app.Server.Get("/{$}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("GETTTT")
		w.WriteHeader(http.StatusOK)
		w.Header().Set(types.CONTENT_TYPE, types.APPLICATION_HTML)
		app.Handler.RenderView(w, r, app.Handler.GetRouteData(handlers.INDEX), views.SignIn())
	})

	wg.Go(func() {
		if err = app.Server.ListenAndServe(); err != nil && err == http.ErrServerClosed {
			app.Log.LogErrorf("Main::main()::serverClosed::%+v", err.Error())
			cancel()
		}
	})

	app.Log.LogInfo(&logger.LoggerPayload{
		Message: "Main::main()::Server is running on",
		Value: map[string]string{
			"host": "127.0.0.1",
			"port": "3000",
		},
	})

	<-ctx.Done()
	app.Log.LogInfof("Main::main()::received shutdown signal...")

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 5*time.Second)
	defer shutdownCancel()

	app.Server.Shutdown(shutdownCtx)

	app.Log.LogInfof("Main::main()::Closing database connections...")

	if err = app.Redis.Close(); err != nil {
		app.Log.LogErrorf("Main::main()::Failed to disconnect from redis: %+v", err)
	}
}

// kubectl exec gateway-deployment-5cd7bc7fb4-8ndrz -- curl -I http://localhost:3000/public/css/style.css

// http.Handle("/public/", http.StripPrefix("/public/", http.FileServer(http.Dir("public"))))
// fs := http.FileServer(http.Dir("./public"))
// http.Handle("/public/", http.StripPrefix("/public/", fs))

// time.AfterFunc(time.Second*5, func() {
// 	users := app.Broker.BuildTopic(rmq.USERS_EX, rmq.USERS_QUEUE, rmq.Request)
// 	request := models.CreateNewMessagePayload(events.UserEvents(events.GET_USERS), &models.Params{}, nil, nil)
// 	requstBytes, err := request.Marshall()
// 	res, err := app.Broker.PubSub(users, requstBytes)
// 	if err != nil {
// 		return
// 	}
// 	fmt.Printf("RECEIVED RESPONSE: %+v\n", map[string]any{
// 		"payload": string(res.Payload),
// 		"from":    res.Service,
// 	})
// })

// app.Broker.Listen(app.Broker.BuildTopic(rmq.USERS_EX, rmq.USERS_QUEUE, rmq.Request), func(mp *models.MessagePayload) ([]byte, error) {
// 	// Check if channel is nil or closed
// 	fmt.Printf("Receiver got message: %+v\n", map[string]any{
// 		"event":  mp.Event,
// 		"params": mp.Params,
// 	})
// 	return []byte(`{"status": "testing-success"}`), nil
// })
