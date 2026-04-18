package main

import (
	"context"

	appgateway "github.com/mfduar8766/learnKubernetes/app-gateway"
)

func main() {
	var (
		ctx                     = context.Background()
		app *appgateway.AppDeps = appgateway.New(ctx)()
	)
	app.Start(ctx)
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
