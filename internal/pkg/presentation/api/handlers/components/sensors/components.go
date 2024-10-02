package sensors

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/locale"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

func NewBatteryLevelComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "max-age=86400")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
		w.WriteHeader(http.StatusOK)

		ctx := r.Context()
		id := r.PathValue("id")

		batteryLevelID := fmt.Sprintf("%s/3/9", id)

		data, err := app.GetMeasurementData(ctx, batteryLevelID, application.WithLastN(true), application.WithLimit(1))
		if err != nil {
			http.Error(w, "could not compose view model", http.StatusInternalServerError)
			return
		}

		var v string = "-"
		var u string = ""

		if len(data.Values) > 0 {
			if data.Values[0].Value != nil {
				v = fmt.Sprintf("%0.f", *data.Values[0].Value)
				u = data.Values[0].Unit
			}
		}

		component := components.Text(fmt.Sprintf("%s%s", v, u))
		component.Render(ctx, w)
	}
	return http.HandlerFunc(fn)
}

func NewMeasurementComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	log := logging.GetFromContext(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		//w.Header().Add("Cache-Control", "max-age=60")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
		w.WriteHeader(http.StatusOK)

		//localizer := l10n.For(r.Header.Get("Accept-Language"))
		ctx := logging.NewContextWithLogger(r.Context(), log)
		id := r.URL.Query().Get("sensorMeasurementTypes")

		/*
			Kolla om både timeAt och endTimeAt är satta. Då är timeRel = "between"
			Om bara endTimeAt är satt så är timeRel = "before"
			Om bara timeAt är satt så är timeRel = "after"

			Default borde vara "between" de senaste 24 timmarna.
		*/

		layout := "2006-01-02T15:04"
		t := r.URL.Query().Get("timeAt")
		if t == "" {
			t = time.Now().Add(time.Hour * -24).Format(layout)
		}
		startTime, err := time.Parse(layout, t)
		if err != nil {
			log.Error("failed to parse timeAt")
		}

		et := r.URL.Query().Get("endTimeAt")
		if et == "" {
			et = time.Now().Format(layout)
		}
		endTime, err := time.Parse(layout, et)
		if err != nil {
			log.Error("failed to parse endTimeAt")
		}

		//now := time.Now()
		//today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		measurements, err := app.GetMeasurementData(ctx, id, application.WithLastN(true), application.WithTimeRel("between", startTime, endTime), application.WithLimit(100), application.WithReverse(true))
		if err != nil {
			http.Error(w, "could not fetch measurement data", http.StatusBadRequest)
			return
		}

		dataset := components.NewChartDataset("")

		previousValue := 0
		for _, v := range measurements.Values {
			if dataset.Label == "" {
				dataset.Label = v.Unit
			}

			if v.Value != nil {
				dataset.Add(v.Timestamp.Format(time.DateTime), *v.Value)
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

		component := components.MeasurementChart([]components.ChartDataset{dataset}, true)
		component.Render(ctx, w)
	}

	return http.HandlerFunc(fn)
}
