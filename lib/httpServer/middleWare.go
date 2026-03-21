package httpServer

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/mfduar8766/learnKubernetes/lib/logger"
	"github.com/mfduar8766/learnKubernetes/lib/types"
)

type Limitter struct {
	wg    *sync.WaitGroup
	batch int
	count int
}

type RateLimitter struct {
	requests map[string]*Limitter
}

// implement a rate limitter where we have 10 incoming requests and we only want to accept 5 requests every 5 seconds
func rateLimitter(wg *sync.WaitGroup) {
	requests := []string{"H", "E", "L", "L", "O", "E", "L", "H", "L", "O", "H", "E", "L", "L", "O"}
	var data chan string = make(chan string)
	wg.Go(func() {
		defer close(data)
		for _, r := range requests {
			data <- r
		}
	})
	wg.Go(func() {
		ticker := time.NewTicker(time.Second * 5)
		defer ticker.Stop()
		batch := 5
		count := 0
		for r := range data {
			fmt.Printf("Handling Request: %+v\n", r)
			count++

			if batch == count {
				fmt.Println("---Request Limit Reached---")
				<-ticker.C
				count = 0
			}
		}
	})
	wg.Wait()
	fmt.Println("All requests processed.")
}

func RequestValidation(ctx *Ctx, log *logger.Logger, w http.ResponseWriter, r *http.Request) error {
	// var user User
	// Use json.NewDecoder to decode the body into the struct
	// err := json.NewDecoder(r.Body).Decode(&user)
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusBadRequest)
	// 	return nil
	// }

	// Remember to close the request body
	// defer r.Body.Close()
	return nil
}

func Auth(ctx *Ctx, log *logger.Logger, w http.ResponseWriter, r *http.Request) error {
	tokenHeader := r.Header.Get(types.HEADER_TOKEN)
	if len(tokenHeader) == 0 {
		return fmt.Errorf("token is empty")
	} else {
		if tokenHeader == "SOME_TOKEN" {
			ctx.SetCtxValue(types.HEADER_TOKEN, tokenHeader)
			return nil
		}
		return fmt.Errorf("incorrect token...")
	}
}
