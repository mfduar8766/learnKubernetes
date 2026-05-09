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
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mfduar8766/learnKubernetes/app-gateway/handlers"
	"github.com/mfduar8766/learnKubernetes/app-gateway/views/common"
	dashboard "github.com/mfduar8766/learnKubernetes/app-gateway/views/dashBoard"
	"github.com/mfduar8766/learnKubernetes/lib/db/redisDb"
	"github.com/mfduar8766/learnKubernetes/lib/httpServer"
	"github.com/mfduar8766/learnKubernetes/lib/logger"
	"github.com/mfduar8766/learnKubernetes/lib/transport"
	"github.com/mfduar8766/learnKubernetes/lib/types"
	"github.com/mfduar8766/learnKubernetes/lib/utils"
	"github.com/redis/go-redis/v9"
)

type AppDeps struct {
	server      httpServer.IServer
	redis       *redis.Client
	handler     handlers.IRequestHandler
	broker      transport.ITransport
	log         logger.ILogger
	tailWindCmd *exec.Cmd
}

func New(ctx context.Context) func() *AppDeps {
	return sync.OnceValue(func() *AppDeps {
		log := logger.NewLogger(types.APP_GATE_WAY)
		redisClient := redisDb.ConnectToRedis(ctx, log)
		broker := transport.New(log)
		err := broker.Connect(ctx, types.APP_GATE_WAY, false)
		if err != nil {
			log.LogFatalf("AppGateWay::New()::received error: %+s", err.Error())
			panic(err)
		}
		srv := httpServer.New(ctx, log, utils.GetHostPort(types.APP_GATE_WAY))
		handler := handlers.New(srv.GetCtx(), broker, log)
		handler.Subscribe(broker.BuildTopic(transport.TOPIC_TYPE_REQUEST, "users"), broker.BuildTopic(transport.TOPIC_TYPE_EVENT, "users"))
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
	var archSuffix string
	switch runtime.GOARCH {
	case "amd64":
		archSuffix = "x64"
	case "arm64":
		archSuffix = "arm64"
	default:
		panic(fmt.Errorf("Main::setUpTailWind()::Failed to download tailwaind exe. Unsupported architecture: %s", runtime.GOARCH))
	}
	fileURL := fmt.Sprintf("https://github.com/tailwindlabs/tailwindcss/releases/download/v4.2.2/tailwindcss-%s-%s", runtime.GOOS, archSuffix)
	var fileName = "tailwindcss"
	if _, err := os.Stat(fileName); err != nil {
		a.log.LogInfof("Main::setUpTailWind()::%s file does not exist. Downloading latest build...", fileName)
		res, err := http.Get(fileURL)
		if err != nil {
			a.log.LogErrorf("Main::setUpTailWind()::Failed to download tailwind exe: %+v", err)
			return
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
		"-i", fmt.Sprintf("%s/css/index.css", types.PUBLIC),
		"-o", fmt.Sprintf("%s/css/style.css", types.PUBLIC),
		"--content", "views/**/*.templ",
	}

	// ALWAYS do an initial build first, regardless of Env
	a.log.LogInfof("Performing initial Tailwind build...")
	initialCmd := exec.Command("./tailwindcss", args...)
	if out, err := initialCmd.CombinedOutput(); err != nil {
		a.log.LogErrorf("Initial Tailwind build failed: %v\n%s", err, string(out))
	} else {
		a.log.LogInfof("Initial CSS generated.")
	}

	if env == types.PROD_ENV {
		return
	}

	watchArgs := append(args, "--watch")
	a.tailWindCmd = exec.Command("./tailwindcss", watchArgs...)
	a.tailWindCmd.Stdout = os.Stdout
	a.tailWindCmd.Stderr = os.Stderr

	if err := a.tailWindCmd.Start(); err != nil {
		a.log.LogErrorf("Failed to start Tailwind watcher: %v", err)
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
		a.handler.Unsubscribe(a.broker.BuildTopic(transport.TOPIC_TYPE_REQUEST, "users"), a.broker.BuildTopic(transport.TOPIC_TYPE_EVENT, "users"))
		err := a.broker.Close()
		if err != nil {
			a.log.LogErrorf("AppGateWay::Start()::Failed to disconnect from broker: %+v", err)
		}
		cancel()
		a.log.Close()
	}()

	a.server.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		if a.redis == nil || a.broker == nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "%+v", errors.New("failed to connect to dbs..."))
			return
		}

		if _, err := os.Stat(fmt.Sprintf("%s/css/style.css", types.PUBLIC)); os.IsNotExist(err) {
			a.log.LogErrorf("AppGateWay::Start()::tailWind is not ready yet err: %s", err.Error())
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "CSS build in progress...")
			return
		}

		if !strings.Contains(r.Header.Get(types.HEADER_USER_AGENT), "kube") {
			a.log.LogInfo(&logger.LoggerPayload{
				Message: "AppGateWay::Start()::received readiness check on",
				Value: map[string]any{
					"host":    r.Host,
					"headers": r.Header,
					"method":  r.Method,
				},
			})
		}
	})

	fileServer := http.FileServer(http.Dir("public"))
	// IN LATEST Go WE CANNOT USE /public/*
	a.server.Get(fmt.Sprintf("/%s/{file...}", types.PUBLIC), func(w http.ResponseWriter, r *http.Request) {
		a.log.LogInfof("Serving static file: %s", r.URL.Path)
		http.StripPrefix("/public/", fileServer).ServeHTTP(w, r)
	})

	a.server.Get(types.API_ENDPOINT, func(w http.ResponseWriter, r *http.Request) {
		a.handler.ProcessRequests(w, r)
	}, httpServer.RequestValidation, httpServer.Auth)

	a.server.Post(types.API_ENDPOINT, func(w http.ResponseWriter, r *http.Request) {
		a.handler.ProcessRequests(w, r)
	}, httpServer.RequestValidation, httpServer.Auth)

	a.server.Get("/{$}", func(w http.ResponseWriter, r *http.Request) {
		a.handler.RenderView(w, r, a.handler.GetRouteData(handlers.INDEX_ROUTE), common.LogInForm(types.API_ENDPOINT))
	})

	a.server.Get(handlers.DASH_BOARD_ROUTE, func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("Received request for dashBoard: %+v\n", r.Header)
		// a.handler.RenderView(w, r, a.handler.GetRouteData(handlers.DASH_BOARD_ROUTE), views.Home())
		a.handler.RenderView(w, r, a.handler.GetRouteData(handlers.DASH_BOARD_ROUTE), dashboard.DashBoard(nil))
	}, httpServer.Auth)

	wg.Go(func() {
		if err := a.server.ListenAndServe(); err != nil && err == http.ErrServerClosed {
			a.log.LogErrorf("AppGateWay::Start()::serverClosed::%+v", err.Error())
			cancel()
		}
	})

	a.log.LogInfo(&logger.LoggerPayload{
		Message: "AppGateWay::Start()::Server is running on",
		Value: map[string]any{
			"host": types.LOCAL_HOST,
			"port": utils.GetHostPort(types.APP_GATE_WAY),
		},
	})

	<-ctx.Done()
	a.log.LogInfof("AppGateWay::Start()::received shutdown signal...")

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
	defer shutdownCancel()

	a.log.LogInfof("AppGateWay::Start()::Closing database connections...")

	if err := a.redis.Close(); err != nil {
		a.log.LogErrorf("AppGateWay::Start()::Failed to disconnect from redis: %+v", err)
	}

	a.server.Shutdown(shutdownCtx)

}
