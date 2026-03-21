package appgateway

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
	server      *httpServer.Server
	redis       *redis.Client
	handler     *handlers.RequestHandler
	broker      *rmq.RMQ
	log         *logger.Logger
	tailWindCmd *exec.Cmd
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
			server:  srv,
			redis:   redisClient,
			handler: handler,
			broker:  broker,
			log:     log,
		}
		appDeps.setUpTailWind()
		appDeps.launchTailWind()
		return &appDeps
	})
}

func (a *AppDeps) setUpTailWind() {
	var fileName = "tailwindcss"
	if _, err := os.Stat(fileName); err != nil {
		a.log.LogInfof("Main::setUpTailWind()::%s file does not exist. Downloading latest build...", fileName)
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
			size, err := io.Copy(file, res.Body)
			if err != nil {
				panic(err)
			}
			a.log.LogInfof("App::setUpTailWind()::received fileSize: %d", size)
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
		"-i", "public/css/index.css",
		"-o", "public/css/style.css",
		"--content", "views/**/*.templ",
	}

	// ALWAYS do an initial build first, regardless of Env
	a.log.LogInfof("⏳ Performing initial Tailwind build...")
	initialCmd := exec.Command("./tailwindcss", args...)
	if out, err := initialCmd.CombinedOutput(); err != nil {
		a.log.LogErrorf("❌ Initial Tailwind build failed: %v\n%s", err, string(out))
	} else {
		a.log.LogInfof("✅ Initial CSS generated.")
	}

	if env == types.PROD_ENV {
		return
	}

	// Now start the watcher for Dev
	watchArgs := append(args, "--watch")
	a.tailWindCmd = exec.Command("./tailwindcss", watchArgs...)
	a.tailWindCmd.Stdout = os.Stdout
	a.tailWindCmd.Stderr = os.Stderr

	if err := a.tailWindCmd.Start(); err != nil {
		a.log.LogErrorf("❌ Failed to start Tailwind watcher: %v", err)
		return
	}
	go func() {
		err := a.tailWindCmd.Wait()
		if err != nil {
			a.log.LogErrorf("Tailwind watcher exited: %v", err)
		}
	}()
}

func (a *AppDeps) Start(parentCtx context.Context) {
	var wg sync.WaitGroup
	ctx, cancel := signal.NotifyContext(parentCtx, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	defer func() {
		if a.tailWindCmd != nil && a.tailWindCmd.Process != nil {
			a.log.LogInfof("Killing Tailwind watcher...")
			a.tailWindCmd.Process.Kill()
		}
		a.broker.Close()
		cancel()
		a.log.Close()
	}()

	a.server.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		if a.redis == nil || a.broker == nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "%+v", errors.New("failed to connect to dbs..."))
			return
		}

		if _, err := os.Stat("public/css/style.css"); os.IsNotExist(err) {
			a.log.LogErrorf("Main::main()::tailWind is not ready yet err: %s", err.Error())
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "CSS build in progress...")
			return
		}

		if !strings.Contains(r.Header.Get(types.USER_AGENT), "kube") {
			a.log.LogInfo(&logger.LoggerPayload{
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
	a.server.Get("/public/{file...}", func(w http.ResponseWriter, r *http.Request) {
		a.log.LogInfof("Serving static file: %s", r.URL.Path)
		http.StripPrefix("/public/", fileServer).ServeHTTP(w, r)
	})

	a.server.Get(types.API_ENDPOINT, func(w http.ResponseWriter, r *http.Request) {
		a.handler.ProcessRequests(w, r)
	}, httpServer.RequestValidation, httpServer.Auth)

	a.server.Get("/{$}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set(types.CONTENT_TYPE, types.APPLICATION_HTML)
		a.handler.RenderView(w, r, a.handler.GetRouteData(handlers.INDEX), views.SignIn())
	})

	wg.Go(func() {
		if err := a.server.ListenAndServe(); err != nil && err == http.ErrServerClosed {
			a.log.LogErrorf("Main::main()::serverClosed::%+v", err.Error())
			cancel()
		}
	})

	a.log.LogInfo(&logger.LoggerPayload{
		Message: "Main::main()::Server is running on",
		Value: map[string]string{
			"host": "127.0.0.1",
			"port": "3000",
		},
	})

	<-ctx.Done()
	a.log.LogInfof("Main::main()::received shutdown signal...")

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 5*time.Second)
	defer shutdownCancel()

	a.server.Shutdown(shutdownCtx)

	a.log.LogInfof("Main::main()::Closing database connections...")

	// if err := a.Redis.Close(); err != nil {
	// 	app.Log.LogErrorf("Main::main()::Failed to disconnect from redis: %+v", err)
	// }
}
