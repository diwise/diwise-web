package sensors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/locale"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func NewTableSensorsComponentHandler(l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.WebApp) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
		w.WriteHeader(http.StatusOK)

		localizer := l10n.For(r.Header.Get("Accept-Language"))

		page := helpers.UrlParamOrDefault(r, "page", "1")
		limit := helpers.UrlParamOrDefault(r, "limit", "30")

		ctx := r.Context()

		_, pages, sensors, _ := getSensors(ctx, page, limit)

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
		"deveui":  lookup("sensorID"),
		"name":    lookup("name"),
		"network": "LoRa",
	}[property]

	return value
}

func NewSens(ctx context.Context, data map[string]any) *sens {
	logging.GetFromContext(ctx).Info("creating sens", "data", data)
	return &sens{data: data}
}

func getSensors(ctx context.Context, page, limit string) (int, int, []components.SensorViewModel, error) {
	count, _ := strconv.ParseInt(limit, 10, 32)
	pageidx, _ := strconv.ParseInt(page, 10, 32)

	token := env.GetVariableOrDefault(ctx, "TOKEN", "invalid")

	url := "https://test.diwise.io/api/v0/devices"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		err = fmt.Errorf("failed to create http request: %w", err)
		return 0, 0, nil, err
	}

	req.Header.Add("Authorization", "Bearer "+token)

	client := http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	resp, err := client.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to retrieve information for device: %w", err)
		return 0, 0, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		err = fmt.Errorf("request failed, not authorized")
		return 0, 0, nil, err
	}

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("request failed with status code %d", resp.StatusCode)
		return 0, 0, nil, err
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("failed to read response body: %w", err)
		return 0, 0, nil, err
	}

	impl := []map[string]any{}

	err = json.Unmarshal(respBody, &impl)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal response body: %w", err)
		return 0, 0, nil, err
	}

	result := []components.SensorViewModel{}
	pageoffset := (pageidx - 1) * count

	for idx := range count {
		if int(pageoffset+idx) >= len(impl) {
			break
		}
		result = append(result, NewSens(ctx, impl[pageoffset+idx]))
	}
	return len(impl), (len(impl) + int(count) - 1) / int(count), result, nil
}
