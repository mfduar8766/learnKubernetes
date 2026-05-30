package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	// rv8 "github.com/go-redis/redis/v8"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/packets"

	// "github.com/mochi-mqtt/server/v2/hooks/storage/redis"
	"github.com/mochi-mqtt/server/v2/listeners"
)

// Used as a custome broker built in Golang.
// In case I want to use this instead of Mosquitto,

type MyHook struct {
	mqtt.HookBase
}

func (h *MyHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnStarted,
		mqtt.OnStopped,
		mqtt.OnConnect,
		mqtt.OnSessionEstablish,
		mqtt.OnSessionEstablished,
		mqtt.OnDisconnect,
		mqtt.OnACLCheck,
	}, []byte{b})
}

func (h *MyHook) ID() string {
	return "my-custom-hook"
}

// 2. Implement OnStarted
func (h *MyHook) OnStarted() {
	fmt.Println("Server has started")
}

// 2. Implement OnStopped
func (h *MyHook) OnStopped() {
	fmt.Println("Server has stopped")
}

func (m *MyHook) Init(config any) error {
	fmt.Printf("Init()::%+v\n", config)
	return nil
}

func (m *MyHook) Stop() error {
	fmt.Println("Stop()...")
	return nil
}

func (m *MyHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	fmt.Printf("onACLCheck()::%+v\n", map[string]any{
		"topic": topic,
		"write": write,
		"ID":    cl.ID,
	})
	return true
}

func (m *MyHook) OnConnect(cl *mqtt.Client, pk packets.Packet) error {
	fmt.Printf("OnConnect()...%+v\n", pk)
	return nil
}

func (m *MyHook) OnSessionEstablish(cl *mqtt.Client, pk packets.Packet) {
	fmt.Printf("OnSessionEstablish()... %+v\n", pk)
}

func (m *MyHook) OnSessionEstablished(cl *mqtt.Client, pk packets.Packet) {
	fmt.Printf("OnSessionEstablished()... %+v\n", pk)
}

func (m *MyHook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	fmt.Println("OnDisconnect()...")
}

func main() {
	// Create signals channel to run server until interrupted
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()

	// Create the new MQTT Server.
	server := mqtt.New(nil)

	// Allow all connections.
	// _ = server.AddHook(new(auth.AllowHook), &auth.Options{})
	err := server.AddHook(new(auth.Hook), &auth.Options{
		Ledger: &auth.Ledger{
			Auth: auth.AuthRules{ // Auth disallows all by default
				{Username: "peach", Password: "password1", Allow: true},
				{Username: "melon", Password: "password2", Allow: true},
				{Username: "user", Password: "password", Allow: true},
				// {Remote: "127.0.0.1:*", Allow: true},
				// {Remote: "localhost:*", Allow: true},
			},
			ACL: auth.ACLRules{ // ACL allows all by default
				{Remote: "127.0.0.1:*"}, // local superuser allow all
				{
					// user melon can read and write to their own topic
					Username: "melon", Filters: auth.Filters{
						"melon/#":   auth.ReadWrite,
						"updates/#": auth.WriteOnly, // can write to updates, but can't read updates from others
					},
				},
				{
					// Otherwise, no clients have publishing permissions
					Filters: auth.Filters{
						"#":         auth.ReadOnly,
						"updates/#": auth.Deny,
					},
				},
			},
		},
	})

	server.AddHook(new(MyHook), nil)

	// err = server.AddHook(new(redis.Hook), &redis.Options{
	// 	Options: &rv8.Options{
	// 		Addr:     "localhost:6379", // default redis address
	// 		Password: "",               // your password
	// 		DB:       0,                // your redis db
	// 	},
	// })
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// Create a TCP listener on a standard port.
	tcp := listeners.NewTCP(listeners.Config{ID: "t1", Address: ":1883"})
	err = server.AddListener(tcp)
	if err != nil {
		panic(err)
	}

	go func() {
		err := server.Serve()
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Run server until interrupted
	<-done
	server.Close()

	// Cleanup
}
