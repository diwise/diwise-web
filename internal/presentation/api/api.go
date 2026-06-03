package api

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/application"
	"github.com/diwise/diwise-web/internal/presentation/api/authz"
	"github.com/diwise/diwise-web/internal/presentation/api/handlers/admin"
	"github.com/diwise/diwise-web/internal/presentation/api/handlers/home"
	"github.com/diwise/diwise-web/internal/presentation/api/handlers/sensors"
	"github.com/diwise/diwise-web/internal/presentation/api/handlers/things"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/presentation/web/components/shared/ui/toast"
	webutils "github.com/diwise/diwise-web/internal/presentation/web/utils"
	frontendtoolkit "github.com/diwise/frontend-toolkit"

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

func NewAuthzDeniedHandler(redirectURL string, l10n frontendtoolkit.LocaleBundle) authz.DeniedHandler {
	return func(w http.ResponseWriter, r *http.Request, denial authz.Denial) {
		switch denial.Status {
		case http.StatusUnauthorized:
			redirectForAuth(w, r, redirectURL)
		case http.StatusForbidden:
			if helpers.IsHxRequest(r) {
				localizer := l10n.For(r.Header.Get("Accept-Language"))
				writeAuthzDeniedToast(
					r.Context(),
					w,
					localizer.Get("missingpermission"),
					localizer.Get("missingpermissiondescription"),
				)
				return
			}

			redirectForAuth(w, r, redirectURL)
		default:
			http.Error(w, http.StatusText(denial.Status), denial.Status)
		}
	}
}

func writeAuthzDeniedToast(ctx context.Context, w http.ResponseWriter, title, message string) {
	component := toast.Toast(toast.Props{
		Title:         strings.TrimSpace(title),
		Description:   strings.TrimSpace(message),
		Variant:       toast.VariantError,
		Dismissible:   true,
		Icon:          true,
		ShowIndicator: true,
		Position:      toast.PositionBottomRight,
	})

	writeToastResponse(ctx, w, "#app-toast", component)
}

func writeToastResponse(ctx context.Context, w http.ResponseWriter, target string, component templ.Component) {
	var buf bytes.Buffer
	if err := component.Render(ctx, &buf); err != nil {
		http.Error(w, "could not render toast", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Retarget", target)
	w.Header().Set("HX-Reswap", "beforeend")
	w.Header().Set("HX-Replace-Url", "false")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// HTMX does not swap 4xx responses by default, so return 200 for the toast fragment.
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

func redirectForAuth(w http.ResponseWriter, r *http.Request, redirectURL string) {
	if helpers.IsHxRequest(r) {
		w.Header().Set("HX-Redirect", redirectURL)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
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

func Auth(authorize func(http.Handler) http.Handler, next http.Handler) http.Handler {
	return authorize(next)
}

func RegisterHandlers(
	ctx context.Context,
	mux *http.ServeMux,
	middleware []func(http.Handler) http.Handler,
	authorizer authz.Authorizer,
	app *application.App,
	assetPath string,
) error {
	if authorizer == nil {
		return errors.New("api access authorizer is required")
	}

	r := http.NewServeMux()
	wrap := func(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
		for _, mw := range slices.Backward(middlewares) {
			handler = mw(handler)
		}

		return handler
	}

	assetLoader, _ := assets.NewLoader(ctx,
		assets.BasePath(assetPath), assets.Logger(logging.GetFromContext(ctx)),
	)
	webutils.ScriptURL = func(path string) string {
		return assetLoader.Load(strings.TrimPrefix(path, "/assets")).Path()
	}

	//requiredscopes
	readSensor := authorizer.RequireAccess(authz.ReadSensors)
	updateSensor := authorizer.RequireAccess(authz.UpdateSensors)
	readSensorTenant := authorizer.RequireTenantAccess(authz.ReadSensors, NewTenantResolverFromSensorPath(app))
	updateSensorTenant := authorizer.RequireTenantAccess(authz.UpdateSensors, NewTenantResolverFromSensorPath(app))
	readThing := authorizer.RequireAccess(authz.ReadThings)
	createThing := authorizer.RequireAccess(authz.CreateThings)
	readThingTenant := authorizer.RequireTenantAccess(authz.ReadThings, NewTenantResolverFromThingsPath(app))
	updateThingTenant := authorizer.RequireTenantAccess(authz.UpdateThings, NewTenantResolverFromThingsPath(app))
	deleteThingTenant := authorizer.RequireTenantAccess(authz.DeleteThings, NewTenantResolverFromThingsPath(app))
	updateThingTenantFromQuery := authorizer.RequireTenantAccess(authz.UpdateThings, NewTenantResolverFromThingsQuery(app))
	adminAccess := authorizer.RequireAccess(authz.Admin)

	l10n := locale.NewLocalizer(assetPath, "sv", "en")

	//Home
	next := home.NewHomePage(ctx, l10n, assetLoader.Load, app)
	r.Handle("GET /", Auth(authorizer.Authenticate(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		next(w, r)
	})))

	r.Handle("GET /home", Auth(readSensor, home.NewHomePage(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /components/home/statistics", RequireHX(Auth(readSensor, home.NewOverviewCardsHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("GET /components/home/usage", RequireHX(Auth(readSensor, home.NewUsageHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("GET /components/tables/alarms", RequireHX(Auth(readSensor, home.NewAlarmsTable(ctx, l10n, assetLoader.Load, app))))

	//Sensors
	r.Handle("GET /sensors", Auth(readSensor, sensors.NewSensorsPage(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /sensors/{id}", Auth(readSensorTenant, sensors.NewSensorDetailsPage(ctx, l10n, assetLoader.Load, app)))
	r.Handle("POST /sensors/{id}", Auth(updateSensorTenant, sensors.NewSaveSensorDetailsPage(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /components/sensors/{id}/attach", RequireHX(Auth(updateSensorTenant, sensors.NewAttachSensorDialogHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("POST /components/sensors/{id}/attach", RequireHX(Auth(updateSensorTenant, sensors.NewAttachSensorDialogHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("GET /components/sensors/{id}/detach", RequireHX(Auth(updateSensorTenant, sensors.NewDetachSensorDialogHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("POST /components/sensors/{id}/detach", RequireHX(Auth(updateSensorTenant, sensors.NewDetachSensorDialogHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("GET /components/sensors/attach/search-options", RequireHX(Auth(updateSensor, sensors.NewAttachSensorSearchOptionsHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("GET /components/sensors/list", RequireHX(Auth(readSensor, sensors.NewSensorsDataList(ctx, l10n, assetLoader.Load, app))))
	r.Handle("GET /components/sensors/{id}/status", RequireHX(Auth(readSensorTenant, sensors.NewStatusChartsComponentHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("GET /components/measurements", RequireHX(sensors.NewMeasurementComponentHandler(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /components/sensors/edit/measurement-types", RequireHX(sensors.NewMeasurementTypesComponentHandler(ctx, l10n, assetLoader.Load, app)))

	//Things
	r.Handle("GET /things", Auth(readThing, things.NewThingsPage(ctx, l10n, assetLoader.Load, app)))
	r.Handle("POST /things", Auth(createThing, things.NewCreateThingPage(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /things/{id}", Auth(readThingTenant, things.NewThingDetailsPage(ctx, l10n, assetLoader.Load, app)))
	r.Handle("POST /things/{id}", Auth(updateThingTenant, things.NewSaveThingDetailsPage(ctx, l10n, assetLoader.Load, app)))
	r.Handle("POST /things/{id}/delete", Auth(deleteThingTenant, things.NewDeleteThingDetailsPage(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /components/things/new", RequireHX(Auth(createThing, things.NewThingComponentHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("GET /components/things/{id}/measurements", RequireHX(Auth(readThingTenant, things.NewThingMeasurementComponentHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("GET /components/things/search-compatible-sensor-options", RequireHX(Auth(updateThingTenantFromQuery, things.NewCompatibleSensorSearchOptionsHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("GET /components/things/list", RequireHX(Auth(readThing, things.NewThingsDataList(ctx, l10n, assetLoader.Load, app))))

	//Admin
	r.Handle("GET /admin", Auth(adminAccess, admin.NewAdminPage(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /admin/export", Auth(adminAccess, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	})))
	r.Handle("POST /admin/import", Auth(adminAccess, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	})))

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
	handler = wrap(handler, middleware...)

	mux.Handle("GET /", handler)
	mux.Handle("POST /", handler)
	mux.Handle("DELETE /", handler)

	return nil
}
