package devmode

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/diwise/diwise-web/internal/pkg/application"
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

func NewAdminTenantsHandler(ctx context.Context) http.HandlerFunc {
	logger := logging.GetFromContext(ctx)

	return func(w http.ResponseWriter, r *http.Request) {

		logger.Info("DEVMODE ADMIN REQUEST", "url", r.URL.String())

		w.Header()["Content-Type"] = []string{"application/json"}
		w.WriteHeader(http.StatusOK)

		response := application.ApiResponse{}
		err := json.Unmarshal([]byte(adminTentantsJsonFormat), &response)
		if err != nil {
			logger.Error("DEVMODE ADMIN ERROR", "error", err)
			http.Error(w, "could not render admin", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(&response)

	}
}

var adminTentantsJsonFormat = `
{
    "data": [
        "default"
    ]
}
`
