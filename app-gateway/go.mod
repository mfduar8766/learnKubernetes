module github.com/mfduar8766/learnKubernetes/app-gateway

go 1.27.0

require (
	github.com/a-h/templ v0.3.1020
	github.com/mfduar8766/learnKubernetes/lib v0.0.0
	github.com/redis/go-redis/v9 v9.22.0
	github.com/stretchr/testify v1.12.1
	google.golang.org/protobuf v1.36.12
)

require (
	dario.cat/mergo v1.0.2 // indirect
	github.com/a-h/parse v0.0.0-20250122154542-74294addb73e // indirect
	github.com/air-verse/air v1.64.5 // indirect
	github.com/andybalholm/brotli v1.2.0 // indirect
	github.com/bep/godartsass/v2 v2.5.0 // indirect
	github.com/bep/golibsass v1.2.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cli/browser v1.3.0 // indirect
	github.com/eclipse/paho.golang v0.23.0 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gohugoio/hugo v0.149.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/natefinch/atomic v1.0.1 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/spf13/afero v1.14.0 // indirect
	github.com/spf13/cast v1.9.2 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	github.com/tdewolff/parse/v2 v2.8.3 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/mod v0.39.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	golang.org/x/tools v0.49.0 // indirect
)

tool (
	github.com/a-h/templ/cmd/templ
	github.com/air-verse/air
)

// Replace the lib module with a local path for development purposes.
//This allows go work to not treat this as a package and instead use
// the local files directly, which is useful for development and testing.
replace github.com/mfduar8766/learnKubernetes/lib => ../lib
