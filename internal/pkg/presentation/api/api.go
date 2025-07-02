package api

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/handlers/components/admin"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/handlers/components/home"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/handlers/components/sensors"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/handlers/components/things"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"

	"github.com/diwise/frontend-toolkit/pkg/assets"
	"github.com/diwise/frontend-toolkit/pkg/locale"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

type writerMiddleware struct {
	rw      http.ResponseWriter
	nocache bool

	contentLength int
	statusCode    int
	isStream      bool
}

func (w *writerMiddleware) disableCache() {
	if w.nocache {
		const CacheHeader string = "Cache-Control"
		currentValue, exists := w.rw.Header()[CacheHeader]
		// Only set no-store if the endpoint hasn't already set immutable
		if !exists || !strings.Contains(currentValue[0], "immutable") {
			w.rw.Header()[CacheHeader] = []string{"no-store"}
		}
	}
}

func (w *writerMiddleware) Flush() {
	f, ok := w.rw.(http.Flusher)
	if ok {
		f.Flush()
	}
}

func (w *writerMiddleware) Header() http.Header {
	return w.rw.Header()
}

func (w *writerMiddleware) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.rw.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("writer middleware does not implement http.Hijacker")
	}
	return h.Hijack()
}

func (w *writerMiddleware) Write(data []byte) (int, error) {
	if w.statusCode == 0 && !w.isStream {
		fmt.Println("write wo header!")
	}

	if w.nocache && w.contentLength == 0 {
		w.disableCache()
	}

	count, err := w.rw.Write(data)
	if err == nil {
		w.contentLength += count
	}
	return count, err
}

func (w *writerMiddleware) WriteHeader(statusCode int) {
	if w.statusCode != 0 {
		return
	}

	if w.nocache {
		w.disableCache()
	}

	w.statusCode = statusCode
	w.rw.WriteHeader(statusCode)
}

func Logger(ctx context.Context) func(http.Handler) http.Handler {
	log := logging.GetFromContext(ctx)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wmw := &writerMiddleware{
				rw:       w,
				isStream: len(r.Header["Accept"]) > 0 && strings.Contains(r.Header["Accept"][0], "text/event-stream"),
			}
			start := time.Now()

			ctx := logging.NewContextWithLogger(r.Context(), log)
			r = r.WithContext(ctx)

			next.ServeHTTP(wmw, r)
			duration := time.Since(start)

			if wmw.statusCode < http.StatusBadRequest {
				log.Debug("served http request", "method", r.Method, "path", r.URL.Path, "status", wmw.statusCode, "duration", duration.String())
			} else if wmw.statusCode < http.StatusInternalServerError {
				log.Warn("served http request", "method", r.Method, "path", r.URL.Path, "status", wmw.statusCode, "duration", duration.String())
			} else {
				log.Error("served http request", "method", r.Method, "path", r.URL.Path, "status", wmw.statusCode, "duration", duration.String())
			}
		})
	}
}

func RequireHX(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isHxRequest := r.Header.Get("HX-Request")
		if isHxRequest != "true" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func VersionReloader(version string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if helpers.IsHxRequest(r) && strings.HasPrefix(r.URL.Path, "/version") {
				if strings.Compare(r.URL.Path, "/version/"+version) != 0 {
					currentURL := r.Header.Get("HX-Current-URL")
					if currentURL == "" {
						currentURL = "/"
					}
					w.Header().Set("HX-Redirect", currentURL)
				}

				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RegisterHandlers(ctx context.Context, mux *http.ServeMux, middleware []func(http.Handler) http.Handler, app *application.App, assetPath string) error {

	r := http.NewServeMux()

	assetLoader, _ := assets.NewLoader(ctx,
		assets.BasePath(assetPath), assets.Logger(logging.GetFromContext(ctx)),
	)

	l10n := locale.NewLocalizer(assetPath, "sv", "en")
	// home
	r.Handle("GET /", func() http.Handler {
		// GET / catches ALL routes that no other handler matches, so we need to make sure that
		// we only serve the homepage when the path actually IS / (or /home as handled below).
		next := home.NewHomePage(ctx, l10n, assetLoader.Load, app)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			next(w, r)
		})
	}())
	r.HandleFunc("GET /home", home.NewHomePage(ctx, l10n, assetLoader.Load, app))
	r.Handle("GET /components/home/statistics", RequireHX(home.NewOverviewCardsHandler(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /components/home/usage", RequireHX(home.NewUsageHandler(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /components/tables/alarms", RequireHX(home.NewAlarmsTable(ctx, l10n, assetLoader.Load, app)))

	// things
	r.HandleFunc("GET /things", things.NewThingsPage(ctx, l10n, assetLoader.Load, app))
	r.HandleFunc("POST /things", things.NewCreateThingComponentHandler(ctx, l10n, assetLoader.Load, app))
	r.HandleFunc("GET /things/{id}", things.NewThingDetailsPage(ctx, l10n, assetLoader.Load, app))
	r.HandleFunc("DELETE /things/{id}", things.DeleteThingComponentHandler(ctx, l10n, assetLoader.Load, app))

	//things - components
	r.Handle("GET /components/things", RequireHX(things.NewThingComponentHandler(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /components/things/{id}", RequireHX(things.NewThingDetailsComponentHandler(ctx, l10n, assetLoader.Load, app)))
	r.Handle("POST /components/things/{id}", RequireHX(things.NewThingDetailsComponentHandler(ctx, l10n, assetLoader.Load, app)))
	r.Handle("DELETE /components/things/{id}", RequireHX(things.NewThingDetailsComponentHandler(ctx, l10n, assetLoader.Load, app)))

	r.Handle("GET /components/tables/things", RequireHX(things.NewThingsTable(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /components/things/list", RequireHX(things.NewThingsDataList(ctx, l10n, assetLoader.Load, app)))

	// sensors
	r.HandleFunc("GET /sensors", sensors.NewSensorsPage(ctx, l10n, assetLoader.Load, app))
	r.HandleFunc("GET /sensors/{id}", sensors.NewSensorDetailsPage(ctx, l10n, assetLoader.Load, app))
	
	r.Handle("GET /components/sensors/details", RequireHX(sensors.NewSensorDetailsComponentHandler(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /components/sensors/details/edit", RequireHX(sensors.NewEditSensorDetailsComponentHandler(ctx, l10n, assetLoader.Load, app)))
	r.HandleFunc("POST /components/sensors/details", sensors.NewSaveSensorDetailsComponentHandler(ctx, l10n, assetLoader.Load, app))
	//r.Handle("GET /components/sensors/{id}/batterylevel", RequireHX(sensors.NewBatteryLevelComponentHandler(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /components/tables/sensors", RequireHX(sensors.NewSensorsTable(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /components/sensors/list", RequireHX(sensors.NewSensorsDataList(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /components/sensors/status/{id}", RequireHX(sensors.NewStatusChartsComponentHandler(ctx, l10n, assetLoader.Load, app)))
	//measurements
	r.Handle("GET /components/measurements", RequireHX(sensors.NewMeasurementComponentHandler(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /components/things/measurements/{id}", RequireHX(things.NewMeasurementComponentHandler(ctx, l10n, assetLoader.Load, app)))
	// admin
	r.Handle("GET /components/admin/types", RequireHX(admin.NewMeasurementTypesComponentHandler(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /error", admin.NewErrorPage(ctx, l10n, assetLoader.Load, app))
	r.Handle("GET /admin", admin.NewAdminPage(ctx, l10n, assetLoader.Load, app))

	r.HandleFunc("GET /admin/export", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		if !query.Has("export") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if !query.Has("accept") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if !query.Has("redirected") {
			query.Set("redirected", "true")
			redirect := fmt.Sprintf("/admin/export?%s", query.Encode())
			w.Header().Set("HX-Redirect", redirect)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(""))
			return
		}

		b, err := app.Export(r.Context(), query)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		w.Header().Set("Content-Type", query.Get("accept"))
		w.WriteHeader(http.StatusOK)
		w.Write(b)
	})
	r.HandleFunc("POST /admin/import", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		defer r.Body.Close()

		contentType := r.Header.Get("Content-Type")
		if !strings.Contains(contentType, "multipart/form-data") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		f, _, err := r.FormFile("file")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		t := r.FormValue("type")

		err = app.Import(ctx, t, f)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	})

	// TODO: Move this handler to a place of its own
	r.Handle("GET /events/{version}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		out, ok := w.(http.Flusher)

		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		log := logging.GetFromContext(r.Context())

		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		const eventFmt string = "event: %s\ndata: %s\n\n"

		log.Info("comparing versions", "client", r.PathValue("version"), "mine", helpers.GetVersion(ctx))

		if r.PathValue("version") != helpers.GetVersion(ctx) {
			log.Warn("client is out of date, sending upgrade and goodbye messages")
			fmt.Fprintf(w, eventFmt, "upgrade", helpers.GetVersion(ctx))
			out.Flush()
			fmt.Fprintf(w, eventFmt, "goodbye", "see you soon")
			out.Flush()

			select {
			case <-time.After(time.Second):
				return
			case <-r.Context().Done():
				return
			}
		}

		log.Info("client connected, sending hello")
		fmt.Fprintf(w, eventFmt, "hello", "version handshake ok")
		out.Flush()

		tmr := time.NewTicker(5 * time.Second)

		for {
			select {
			case t := <-tmr.C:
				fmt.Fprintf(w, eventFmt, "tick", t.Format(time.RFC3339Nano))
				out.Flush()
			case <-r.Context().Done():
				log.Info("sse client closed the connection")
				return
			case <-ctx.Done():
				log.Info("we are closing down, sending goodbye to client")
				fmt.Fprintf(w, eventFmt, "goodbye", "system closing down")
				out.Flush()
				return
			}
		}
	}))

	// Handle requests for leaflet images /assets/<leafletcss-sha>/images/<image>.png
	leafletSHA := assetLoader.Load("/css/leaflet.css").SHA256()

	assets.RegisterEndpoints(ctx, assetLoader, assets.WithMux(r),
		assets.WithImmutableExpiry(48*time.Hour),
		assets.WithRedirect("/favicon.ico", "/icons/favicon.ico", http.StatusFound),
		assets.WithRedirect(
			fmt.Sprintf("/assets/%s/images/{img}", leafletSHA), "/images/leaflet-{img}", http.StatusMovedPermanently,
		),
	)

	var handler http.Handler = r

	// wrap the mux with any passed in middleware handlers
	for _, mw := range slices.Backward(middleware) {
		handler = mw(handler)
	}

	mux.Handle("GET /", handler)
	mux.Handle("POST /", handler)
	mux.Handle("DELETE /", handler)

	return nil
}
