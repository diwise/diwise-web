package devmode

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

func NewAdminHandler(ctx context.Context) http.HandlerFunc {
	logger := logging.GetFromContext(ctx)

	return func(w http.ResponseWriter, r *http.Request) {

		logger.Info("DEVMODE ADMIN REQUEST", "url", r.URL.String())

		w.Header()["Content-Type"] = []string{"application/json"}
		w.WriteHeader(http.StatusOK)

		json.NewEncoder(w).Encode(&emptyResponse)
	}
}
