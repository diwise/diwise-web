package application

import "context"

type WebApp interface{}

type app struct{}

func New(ctx context.Context) (WebApp, error) {
	return &app{}, nil
}
