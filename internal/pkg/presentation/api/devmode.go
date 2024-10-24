package api

import (
	"context"
	"net/http"

	"github.com/diwise/diwise-web/internal/pkg/presentation/api/handlers/devmode"
)

const DevModePrefix string = "/devmode"

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

	nocache := http.NewServeMux()
	nocache.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		wmw := &writerMiddleware{rw: w, nocache: true}
		mux.ServeHTTP(wmw, r)
	})

	nologin := http.NewServeMux()
	nologin.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Authorization", "Bearer devmode")
		nocache.ServeHTTP(w, r)
	})

	return nologin
}
