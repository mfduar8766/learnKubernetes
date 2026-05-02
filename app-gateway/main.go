package main

import (
	"context"
)

func main() {
	var (
		ctx          = context.Background()
		app *AppDeps = New(ctx)()
	)
	app.Start(ctx)
}
