package sensors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/authz"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/locale"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func NewSensorDetailsComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.WebApp) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))
		sensorID := r.URL.Query().Get("id")

		ctx := r.Context()

		sensor, err := getSensor(ctx, sensorID)

		if err != nil {
			logging.GetFromContext(ctx).Error("unable to get sensor details", "err", err.Error())
			http.Error(w, "unable to get sensor details", http.StatusInternalServerError)
			return
		}

		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
		w.WriteHeader(http.StatusOK)

		component := components.SensorDetails(localizer, assets, sensor)
		component.Render(ctx, w)
	}

	return http.HandlerFunc(fn)
}

func NewSensorEditorComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.WebApp) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {

		var err error
		var sensor components.SensorViewModel

		ctx := r.Context()
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		if r.Method == http.MethodGet {
			deviceID := r.URL.Query().Get("id")

			sensor, err = getSensor(ctx, deviceID)

			if err != nil {
				logging.GetFromContext(ctx).Error("unable to get sensor details", "err", err.Error())
				http.Error(w, "unable to get sensor details", http.StatusInternalServerError)
				return
			}

			w.Header().Add("Content-Type", "text/html")
			w.Header().Add("Cache-Control", "no-cache")
			w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
			w.WriteHeader(http.StatusOK)

			component := components.EditSensorComponent(localizer, assets, sensor)
			component.Render(ctx, w)

			return
		}

		deviceID := r.PostFormValue("id")
		sensor, err = getSensor(ctx, deviceID)
		if err != nil {
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		//TOOD: set new values and PATCH to backend

		http.Redirect(w, r, "/components/sensors/details?id="+deviceID, http.StatusFound)
	}

	return http.HandlerFunc(fn)
}

func NewTableSensorsComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.WebApp) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
		w.WriteHeader(http.StatusOK)

		localizer := l10n.For(r.Header.Get("Accept-Language"))

		page := helpers.UrlParamOrDefault(r, "page", "1")
		pageSize := helpers.UrlParamOrDefault(r, "limit", "15")

		limit, _ := strconv.Atoi(pageSize)

		offset := func() int {
			p, _ := strconv.Atoi(page)
			return (p - 1) * limit
		}

		ctx := r.Context()

		_, pages, sensors, err := getSensors(ctx, offset(), limit)

		if err != nil {
			logging.GetFromContext(ctx).Error("get sensors error", "err", err.Error())
		}

		ctx = helpers.Decorate(
			ctx,
			components.PageIndex, page,
			components.PageLast, fmt.Sprintf("%d", pages),
			components.PageSize, limit,
		)

		component := components.SensorTable(localizer, assets, sensors)
		component.Render(ctx, w)
	}

	return http.HandlerFunc(fn)
}

type sens struct {
	data map[string]any
}

func getDataValue[T comparable](data map[string]any, path string) T {
	pathParts := strings.Split(path, ".")

	if len(pathParts) == 1 {
		return data[path].(T)
	}

	v := data[pathParts[0]].(map[string]any)
	return getDataValue[T](v, strings.Join(pathParts[1:], "."))
}

func (s *sens) Bool(property string) bool {
	if property == "has-alerts" {
		return getDataValue[float64](s.data, "deviceState.state") > 1.0
	}

	value, ok := s.data[property]
	if !ok {
		return false
	}
	return value.(bool)
}

func (s *sens) Float(property string) float64 {
	if property == "latitude" {
		return getDataValue[float64](s.data, "location.latitude")
	}
	if property == "longitude" {
		return getDataValue[float64](s.data, "location.longitude")
	}
	return 0.0
}

func (s *sens) Date(property, layout string) string {
	if property == "lastseen" {
		value := getDataValue[string](s.data, "deviceState.observedAt")
		tm, err := time.Parse(time.RFC3339, value)
		if err == nil && !tm.IsZero() {
			return tm.Local().Format(layout)
		}
	}

	return ""
}

func (s *sens) String(property string) string {
	lookup := func(p string) string {
		value, ok := s.data[p]
		if !ok {
			return "unknown"
		}
		return value.(string)
	}

	value := map[string]string{
		"deveui":        lookup("sensorID"),
		"id":            lookup("deviceID"),
		"name":          lookup("name"),
		"tenant":        lookup("tenant"),
		"description":   lookup("description"),
		"deviceprofile": getDataValue[string](s.data, "deviceProfile.name"),
		"network":       "LoRa",
	}[property]

	return value
}

func NewSens(ctx context.Context, data map[string]any) *sens {
	logging.GetFromContext(ctx).Info("creating sens", "data", data)
	return &sens{data: data}
}

func getSensor(ctx context.Context, deviceID string) (sensor components.SensorViewModel, err error) {
	res, err := get(ctx, deviceID, url.Values{})
	if err != nil {
		return nil, err
	}

	data := res.Data
	if d, ok := data.(map[string]any); ok {
		return NewSens(ctx, d), nil
	}

	return nil, fmt.Errorf("could not fetch sensor")
}

func getSensors(ctx context.Context, offset, limit int) (int, int, []components.SensorViewModel, error) {
	params := url.Values{}
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("offset", fmt.Sprintf("%d", offset))

	resp, err := get(ctx, "", params)
	if err != nil {
		return 0, 0, []components.SensorViewModel{}, err
	}

	sensors := []components.SensorViewModel{}
	if data, ok := resp.Data.([]any); ok {
		for _, v := range data {
			if m, ok := v.(map[string]any); ok {
				sensors = append(sensors, NewSens(ctx, m))
			}
		}
	}

	return int(resp.Meta.TotalRecords), int(*resp.Meta.Limit), sensors, err
}

func get(ctx context.Context, path string, params url.Values) (*ApiResponse, error) {
	deviceManagementURL := env.GetVariableOrDefault(ctx, "DEV_MGMT_URL", "https://test.diwise.io/api/v0/devices")

	if strings.ContainsAny(path, "/") {
		path = strings.TrimPrefix(path, "/")
		path = strings.TrimSuffix(path, "/")
	}

	u, err := url.Parse(strings.TrimSuffix(fmt.Sprintf("%s/%s", deviceManagementURL, path), "/"))
	if err != nil {
		return nil, err
	}

	u.RawQuery = params.Encode()

	token := authz.Token(ctx)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		err = fmt.Errorf("failed to create http request: %w", err)
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+token)

	client := http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	resp, err := client.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to retrieve information: %w", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		err = fmt.Errorf("request failed, not authorized")
		return nil, err
	}

	if resp.StatusCode > http.StatusIMUsed {
		err = fmt.Errorf("request failed with status code %d", resp.StatusCode)
		return nil, err
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("failed to read response body: %w", err)
		return nil, err
	}

	impl := ApiResponse{}

	err = json.Unmarshal(respBody, &impl)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal response body: %w", err)
		return nil, err
	}

	return &impl, nil
}

type Meta struct {
	TotalRecords uint64  `json:"totalRecords"`
	Offset       *uint64 `json:"offset,omitempty"`
	Limit        *uint64 `json:"limit,omitempty"`
	Count        uint64  `json:"count"`
}

type Links struct {
	Self  *string `json:"self,omitempty"`
	First *string `json:"first,omitempty"`
	Prev  *string `json:"prev,omitempty"`
	Next  *string `json:"next,omitempty"`
	Last  *string `json:"last,omitempty"`
}

type ApiResponse struct {
	Meta  *Meta  `json:"meta,omitempty"`
	Data  any    `json:"data"`
	Links *Links `json:"links,omitempty"`
}
