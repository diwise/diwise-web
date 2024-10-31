package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/diwise/diwise-web/internal/pkg/presentation/api/handlers/devmode"
)

const DevModePrefix string = "/devmode"

func NoCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wmw := &writerMiddleware{
			rw:       w,
			nocache:  true,
			isStream: len(r.Header["Accept"]) > 0 && strings.Contains(r.Header["Accept"][0], "text/event-stream"),
		}
		next.ServeHTTP(wmw, r)
	})
}

func NoLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Authorization", "Bearer devmode")
		next.ServeHTTP(w, r)
	})
}

func InstallDevmodeHandlers(ctx context.Context, mux *http.ServeMux) *http.ServeMux {

	devmux := http.NewServeMux()
	devmux.HandleFunc("GET /admin", devmode.NewAdminHandler(ctx))
	devmux.HandleFunc("GET /admin/tenants", devmode.NewAdminTenantsHandler(ctx))
	devmux.HandleFunc("GET /alarms", devmode.NewAlarmsHandler(ctx))
	devmux.HandleFunc("GET /devices", devmode.NewDevicesHandler(ctx))
	devmux.HandleFunc("GET /measurements", devmode.NewMeasurementsHandler(ctx))
	devmux.HandleFunc("GET /things", devmode.NewThingsHandler(ctx))
	devmux.HandleFunc("GET /things/{id}", devmode.NewThingHandler(ctx))
	devmux.HandleFunc("GET /things/tags", devmode.NewThingsTagsHandler(ctx))
	devmux.HandleFunc("GET /things/types", devmode.NewThingsTypesHandler(ctx))

	mux.Handle("GET "+DevModePrefix+"/", http.StripPrefix(DevModePrefix, devmux))

	return mux
}
