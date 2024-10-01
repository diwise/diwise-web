package things

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/locale"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

func NewMeasurementComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	log := logging.GetFromContext(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		//w.Header().Add("Cache-Control", "max-age=60")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
		w.WriteHeader(http.StatusOK)

		//localizer := l10n.For(r.Header.Get("Accept-Language"))
		ctx := logging.NewContextWithLogger(r.Context(), log)
		thingType := r.PathValue("type")
		if thingType == "" {
			http.Error(w, "no type found in url", http.StatusBadRequest)
			return
		}

		measurementID := r.URL.Query().Get("sensorMeasurementTypes")

		today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.UTC)
		timeAt := getTime(r, "timeAt", today)
		endOfDay := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 23, 59, 59, 0, time.UTC)
		endTimeAt := getTime(r, "endTimeAt", endOfDay)

		params := []application.InputParam{}

		switch thingType {
		case "wastecontainer":
			params = append(params,
				application.WithLastN(true),
				application.WithReverse(true),
				application.WithTimeRel("between", timeAt, endTimeAt),
				application.WithLimit(1000))

		case "passage":
			params = append(params,
				application.WithTimeUnit("hour"),
				application.WithAggrMethods("rate"),
				application.WithBoolValue(true),
				application.WithTimeRel("between", timeAt, endTimeAt),
				application.WithLimit(100))
		default:
			params = append(params,
				application.WithLastN(true),
				application.WithReverse(true),
				application.WithTimeRel("between", timeAt, endTimeAt))
		}

		measurements, err := app.GetMeasurementData(ctx, measurementID, params...)
		if err != nil {
			http.Error(w, "could not fetch measurement data", http.StatusBadRequest)
			return
		}

		dataset := toDataset(measurements.Values)

		var component templ.Component

		switch thingType {
		case "passage":
			component = components.PassagesChart([]components.ChartDataset{dataset})
		case "wastecontainer":
			component = components.WastecontainerChart([]components.ChartDataset{dataset})
		default:
			component = components.MeasurementChart([]components.ChartDataset{dataset}, false)
		}

		component.Render(ctx, w)
	}

	return http.HandlerFunc(fn)
}

func NewCurrentValueComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	log := logging.GetFromContext(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "max-age=60")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
		w.WriteHeader(http.StatusOK)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		ctx := logging.NewContextWithLogger(r.Context(), log)
		thingType := strings.ToLower(r.PathValue("type"))
		if thingType == "" {
			http.Error(w, "no type found in url", http.StatusBadRequest)
			return
		}

		measurementID := r.URL.Query().Get("sensorMeasurementTypes")

		today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.UTC)
		timeAt := getTime(r, "timeAt", today)
		endTimeAt := getTime(r, "endTimeAt", time.Now().UTC())

		params := []application.InputParam{}

		switch thingType {
		case "wastecontainer":
			params = append(params,
				application.WithLastN(true),
				application.WithReverse(true),
				application.WithTimeRel("between", timeAt, endTimeAt))
		case "passage":
			params = append(params,
				application.WithTimeUnit("day"),
				application.WithAggrMethods("rate"),
				application.WithBoolValue(true),
				application.WithTimeRel("between", timeAt, endTimeAt))
		default:
			params = append(params,
				application.WithLastN(true),
				application.WithReverse(true),
				application.WithTimeRel("between", timeAt, endTimeAt))
		}

		measurements, err := app.GetMeasurementData(ctx, measurementID, params...)
		if err != nil {
			http.Error(w, "could not fetch measurement data", http.StatusBadRequest)
			return
		}

		var component templ.Component

		switch thingType {
		case "wastecontainer":
			if len(measurements.Values) == 0 {
				component = components.Text(localizer.Get("noData"))
			} else {
				last := measurements.Values[len(measurements.Values)-1]
				component = components.Text(fmt.Sprintf("%0.f%%", *last.Value))
			}
		case "passage":
			if len(measurements.Values) == 0 {
				component = components.Text(localizer.Get("noData"))
			} else {
				last := measurements.Values[len(measurements.Values)-1]
				component = components.Text(fmt.Sprintf("%d st", last.Count))
			}
		default:

		}

		component.Render(ctx, w)
	}

	return http.HandlerFunc(fn)
}

func toDataset(measurements []application.MeasurementValue) components.ChartDataset {
	dataset := components.NewChartDataset("")
	previousValue := 0
	for _, v := range measurements {
		if dataset.Label == "" {
			dataset.Label = v.Unit
		}

		if v.Value != nil {
			dataset.Add(v.Timestamp.Format(time.DateTime), *v.Value)
		}

		if v.Value == nil && v.Count > 0 {
			dataset.Add(v.Timestamp.Format(time.DateTime), float64(v.Count))
		}

		if v.Value == nil && v.BoolValue != nil {
			vb := 0
			if *v.BoolValue {
				vb = 1
			}

			if vb != previousValue {
				// append value when 0->1 and 1->0
				dataset.Add(v.Timestamp.Format(time.DateTime), previousValue)
				previousValue = vb
			}

			dataset.Add(v.Timestamp.Format(time.DateTime), vb)
		}
	}
	return dataset
}

func getTime(r *http.Request, key string, def time.Time) time.Time {
	layout := "2006-01-02T15:04"

	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	t, err := time.Parse(layout, v)
	if err != nil {
		return def
	}
	return t
}
