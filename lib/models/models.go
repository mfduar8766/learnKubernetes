package models

import "encoding/json"

type Posts struct {
	UserID int    `json:"userId"`
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

type User struct {
	UserID int    `json:"userId"`
	ID     int    `json:"id"`
	Token  string `json:"token"`
}

type Request[T any] struct {
	ApiName string `json:"apiName"`
	Body    T      `json:"body"`
}

func (r *Request[T]) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func (r *Request[T]) UnMarshal(data []byte) error {
	return json.Unmarshal(data, r)
}
