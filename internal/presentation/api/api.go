package api

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/application"
	appclient "github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	"github.com/diwise/diwise-web/internal/presentation/api/handlers/admin"
	"github.com/diwise/diwise-web/internal/presentation/api/handlers/home"
	"github.com/diwise/diwise-web/internal/presentation/api/handlers/sensors"
	"github.com/diwise/diwise-web/internal/presentation/api/handlers/things"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	authcomponents "github.com/diwise/diwise-web/internal/presentation/web/components/features/auth"
	v2layout "github.com/diwise/diwise-web/internal/presentation/web/components/layout"
	webutils "github.com/diwise/diwise-web/internal/presentation/web/utils"

	frontend "github.com/diwise/frontend-toolkit"
	"github.com/diwise/frontend-toolkit/pkg/assets"
	"github.com/diwise/frontend-toolkit/pkg/locale"
	"github.com/diwise/frontend-toolkit/pkg/middleware/csp"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

const (
	ReadSensors   auth.Scope = "sensors.read"
	UpdateSensors auth.Scope = "sensors.update"

	ReadThings   auth.Scope = "things.read"
	CreateThings auth.Scope = "things.create"
	UpdateThings auth.Scope = "things.update"
	DeleteThings auth.Scope = "things.delete"

	Admin auth.Scope = "admin"
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

type responseCapture struct {
	header     http.Header
	body       bytes.Buffer
	statusCode int
}

func newResponseCapture() *responseCapture {
	return &responseCapture{header: make(http.Header)}
}

func (c *responseCapture) Header() http.Header {
	return c.header
}

func (c *responseCapture) Write(data []byte) (int, error) {
	if c.statusCode == 0 {
		c.statusCode = http.StatusOK
	}
	return c.body.Write(data)
}

func (c *responseCapture) WriteHeader(statusCode int) {
	if c.statusCode != 0 {
		return
	}
	c.statusCode = statusCode
}

func AccessDenied(version string, l10n frontend.LocaleBundle, asset frontend.AssetLoaderFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !shouldCaptureForAccessDeniedToast(r) {
				next.ServeHTTP(w, r)
				return
			}

			ctx := appclient.WithAccessDeniedTracker(r.Context())
			r = r.WithContext(ctx)

			capture := newResponseCapture()
			next.ServeHTTP(capture, r)

			// 401: Authentication failure (no token/invalid token) → redirect to /home
			if capture.statusCode == http.StatusUnauthorized {
				redirectToHome(w, r)
				return
			}

			// 403: Authorization failure (valid token but insufficient scopes) → show toast
			if capture.statusCode == http.StatusForbidden {
				writeAccessDeniedToastResponse(w, r, version, l10n, asset)
				return
			}

			// Other errors from internal requests that marked access denied
			if appclient.AccessDenied(ctx) != appclient.AccessDeniedNone {
				writeAccessDeniedToastResponse(w, r, version, l10n, asset)
				return
			}

			writeCapturedResponse(w, capture)
		})
	}
}

func redirectToHome(w http.ResponseWriter, r *http.Request) {
	if strings.EqualFold(r.Header.Get("HX-Request"), "true") {
		w.Header().Set("HX-Redirect", "/home")
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/home", http.StatusFound)
}

func shouldCaptureForAccessDeniedToast(r *http.Request) bool {
	if isEventStream(r) || isUpgradeRequest(r) {
		return false
	}

	return helpers.IsHxRequest(r) || strings.Contains(r.Header.Get("Accept"), "text/html")
}

func isEventStream(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/event-stream")
}

func isUpgradeRequest(r *http.Request) bool {
	return r.Header.Get("Upgrade") != "" || strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

func writeCapturedResponse(w http.ResponseWriter, capture *responseCapture) {
	copyHeaders(w.Header(), capture.Header())

	status := capture.statusCode
	if status == 0 {
		status = http.StatusOK
	}

	w.WriteHeader(status)
	if capture.body.Len() > 0 {
		_, _ = w.Write(capture.body.Bytes())
	}
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		dst.Del(key)
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func writeAccessDeniedToastResponse(w http.ResponseWriter, r *http.Request, version string, l10n frontend.LocaleBundle, asset frontend.AssetLoaderFunc) {
	ctx := r.Context()
	component := authcomponents.AccessDeniedToast()
	status := http.StatusUnauthorized

	if helpers.IsHxRequest(r) {
		w.Header().Set("HX-Retarget", "#toast-container")
		w.Header().Set("HX-Reswap", "innerHTML")
		status = http.StatusOK
	} else {
		localizer := l10n.For(r.Header.Get("Accept-Language"))
		component = v2layout.AccessDeniedShell(version, localizer, asset, component)
	}

	body, err := renderComponent(ctx, component)
	if err != nil {
		logging.GetFromContext(ctx).Error("failed to render access denied toast", "err", err.Error())
		http.Error(w, "access denied", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func renderComponent(ctx context.Context, component templ.Component) ([]byte, error) {
	var buf bytes.Buffer
	ctx = templ.WithNonce(ctx, csp.Nonce(ctx))
	err := component.Render(ctx, &buf)
	return buf.Bytes(), err
}

func RegisterHandlers(ctx context.Context, mux *http.ServeMux, middleware []func(http.Handler) http.Handler, app *application.App, assetPath string, authPolicies io.Reader, opts ...auth.Option) error {

	r := http.NewServeMux()

	policyBytes, err := io.ReadAll(authPolicies)
	if err != nil {
		return fmt.Errorf("failed to read api auth policies: %w", err)
	}

	authenticator, err := auth.NewAuthenticator(
		ctx,
		bytes.NewReader(policyBytes),
		opts...,
	)
	if err != nil {
		return fmt.Errorf("failed to create api authenticator: %w", err)
	}

	optionalAccess := authenticator.OptionalAuth(auth.AnyScope)

	protect := func(scope auth.Scope, next http.Handler) http.Handler {
		return authenticator.RequireAccess(scope)(next)
	}
	protectFunc := func(scope auth.Scope, next http.HandlerFunc) http.Handler {
		return protect(scope, next)
	}

	assetLoader, _ := assets.NewLoader(ctx,
		assets.BasePath(assetPath), assets.Logger(logging.GetFromContext(ctx)),
	)
	webutils.ScriptURL = func(path string) string {
		return assetLoader.Load(strings.TrimPrefix(path, "/assets")).Path()
	}

	l10n := locale.NewLocalizer(assetPath, "sv", "en")
	middleware = append(middleware, AccessDenied(helpers.GetVersion(ctx), l10n, assetLoader.Load))

	r.Handle("GET /", optionalAccess(func() http.Handler {
		next := home.NewHomePage(ctx, l10n, assetLoader.Load, app)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			next(w, r)
		})
	}()))
	r.Handle("GET /home", optionalAccess(home.NewHomePage(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /components/home/statistics", protect(ReadSensors, RequireHX(home.NewOverviewCardsHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("GET /components/home/usage", protect(ReadSensors, RequireHX(home.NewUsageHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("GET /components/tables/alarms", protect(ReadSensors, RequireHX(home.NewAlarmsTable(ctx, l10n, assetLoader.Load, app))))

	r.Handle("GET /sensors", protectFunc(ReadSensors, sensors.NewSensorsPage(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /sensors/{id}", protectFunc(ReadSensors, sensors.NewSensorDetailsPage(ctx, l10n, assetLoader.Load, app)))
	r.Handle("POST /sensors/{id}", protectFunc(UpdateSensors, sensors.NewSaveSensorDetailsPage(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /components/sensors/{id}/attach", protect(UpdateSensors, RequireHX(sensors.NewAttachSensorDialogHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("POST /components/sensors/{id}/attach", protect(UpdateSensors, RequireHX(sensors.NewAttachSensorDialogHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("GET /components/sensors/{id}/detach", protect(UpdateSensors, RequireHX(sensors.NewDetachSensorDialogHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("POST /components/sensors/{id}/detach", protect(UpdateSensors, RequireHX(sensors.NewDetachSensorDialogHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("GET /components/sensors/attach/search-options", protect(UpdateSensors, RequireHX(sensors.NewAttachSensorSearchOptionsHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("GET /components/sensors/list", protect(ReadSensors, RequireHX(sensors.NewSensorsDataList(ctx, l10n, assetLoader.Load, app))))
	r.Handle("GET /components/tables/sensors", protect(ReadSensors, RequireHX(sensors.NewSensorsTable(ctx, l10n, assetLoader.Load, app))))
	r.Handle("GET /components/sensors/{id}/status", protect(ReadSensors, RequireHX(sensors.NewStatusChartsComponentHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("GET /components/measurements", protect(ReadSensors, RequireHX(sensors.NewMeasurementComponentHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("GET /components/sensors/edit/measurement-types", protect(UpdateSensors, RequireHX(sensors.NewMeasurementTypesComponentHandler(ctx, l10n, assetLoader.Load, app))))

	r.Handle("GET /things", protectFunc(ReadThings, things.NewThingsPage(ctx, l10n, assetLoader.Load, app)))
	r.Handle("POST /things", protectFunc(CreateThings, things.NewCreateThingPage(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /things/{id}", protectFunc(ReadThings, things.NewThingDetailsPage(ctx, l10n, assetLoader.Load, app)))
	r.Handle("POST /things/{id}", protectFunc(UpdateThings, things.NewSaveThingDetailsPage(ctx, l10n, assetLoader.Load, app)))
	r.Handle("POST /things/{id}/delete", protectFunc(DeleteThings, things.NewDeleteThingDetailsPage(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /components/things/new", protect(CreateThings, RequireHX(things.NewThingComponentHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("GET /components/things/{id}/measurements", protect(ReadThings, RequireHX(things.NewThingMeasurementComponentHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("GET /components/things/search-compatible-sensor-options", protect(UpdateThings, RequireHX(things.NewCompatibleSensorSearchOptionsHandler(ctx, l10n, assetLoader.Load, app))))
	r.Handle("GET /components/things/list", protect(ReadThings, RequireHX(things.NewThingsDataList(ctx, l10n, assetLoader.Load, app))))

	r.Handle("GET /admin", protect(Admin, admin.NewAdminPage(ctx, l10n, assetLoader.Load, app)))
	r.Handle("GET /admin/export", protectFunc(Admin, func(w http.ResponseWriter, r *http.Request) {
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
	}))
	r.Handle("POST /admin/import", protectFunc(Admin, func(w http.ResponseWriter, r *http.Request) {
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
	}))

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
