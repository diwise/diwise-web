package handlers

import (
	"context"
	"net/http"
)

func NewLivenessHandler(ctx context.Context, isAlive func() error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if err := isAlive(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
