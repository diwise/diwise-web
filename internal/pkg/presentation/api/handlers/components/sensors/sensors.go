package sensors

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/locale"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

func NewSensorDetailsComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		mode := r.URL.Query().Get("mode")
		ctx := r.Context()

		detailsViewModel, err := composeViewModel(ctx, id, app)
		if err != nil {
			http.Error(w, "could not compose view model", http.StatusInternalServerError)
			return
		}

		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
		w.WriteHeader(http.StatusOK)

		if mode == "edit" {
			tenants := app.GetTenants(ctx)
			deviceProfiles := app.GetDeviceProfiles(ctx)

			dp := []components.DeviceProfile{}
			for _, p := range deviceProfiles {
				types := []string{}
				if p.Types != nil {
					types = *p.Types
				}
				dp = append(dp, components.DeviceProfile{
					Name:     p.Name,
					Decoder:  p.Decoder,
					Interval: p.Interval,
					Types:    types,
				})
			}

			detailsViewModel.Organisations = tenants
			detailsViewModel.DeviceProfiles = dp

			component := components.EditSensorDetails(localizer, assets, *detailsViewModel)
			component.Render(ctx, w)
			return
		}

		component := components.SensorDetails(localizer, assets, *detailsViewModel)
		component.Render(ctx, w)
	}

	return http.HandlerFunc(fn)
}

func NewSaveSensorDetailsComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	log := logging.GetFromContext(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		ctx := logging.NewContextWithLogger(r.Context(), log)

		err := r.ParseForm()
		if err != nil {
			http.Error(w, "could not parse form data", http.StatusBadRequest)
			return
		}

		asBool := func(s string) bool {
			return s == "on"
		}

		asFloat := func(s string) (float64, bool) {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				return f, true
			}
			return 0.0, false
		}

		id := r.Form.Get("id")

		if r.Form.Has("save") {
			fields := make(map[string]any)

			for k := range r.Form {
				v := r.Form.Get(k)

				if v == "" {
					continue
				}

				switch k {
				case "id":
					fields["deviceID"] = v
				case "active":
					fields[k] = asBool(v)
				case "longitude":
					if f, ok := asFloat(v); ok {
						fields[k] = f
					}
				case "latitude":
					if f, ok := asFloat(v); ok {
						fields[k] = f
					}
				case "sensorType":
					fields["deviceProfile"] = v
				case "organisation":
					fields["tenant"] = v
				case "measurementType-option[]":
					fields["types"] = r.Form[k]
				default:
					fields[k] = r.Form.Get(k)
				}
			}

			err = app.UpdateSensor(ctx, id, fields)
			if err != nil {
				http.Error(w, "could not update sensor", http.StatusInternalServerError)
				return
			}
		}

		http.Redirect(w, r, "/sensors/"+id, http.StatusFound)
	}

	return http.HandlerFunc(fn)
}

func NewTableSensorsComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	log := logging.GetFromContext(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
		w.WriteHeader(http.StatusOK)

		localizer := l10n.For(r.Header.Get("Accept-Language"))

		pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
		offset, limit := helpers.GetOffsetAndLimit(r)

		ctx := logging.NewContextWithLogger(r.Context(), log)

		sensorResult, err := app.GetSensors(ctx, offset, limit)
		if err != nil {
			http.Error(w, "could not fetch sensors", http.StatusBadRequest)
			return
		}

		listViewModel := components.SensorListViewModel{}

		for _, sensor := range sensorResult.Sensors {
			listViewModel.Sensors = append(listViewModel.Sensors, components.SensorViewModel{
				Active:       sensor.Active,
				DevEUI:       sensor.SensorID,
				DeviceID:     sensor.DeviceID,
				Name:         sensor.Name,
				BatteryLevel: sensor.DeviceStatus.BatteryLevel,
				LastSeen:     sensor.DeviceState.ObservedAt,
				HasAlerts:    false, //TODO: fix this
			})
		}

		renderCtx := helpers.Decorate(
			ctx,
			components.PageIndex, pageIndex,
			components.PageLast, sensorResult.TotalRecords/limit,
			components.PageSize, limit,
		)

		component := components.SensorTable(localizer, assets, listViewModel)
		component.Render(renderCtx, w)
	}

	return http.HandlerFunc(fn)
}

func NewMeasurementComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	log := logging.GetFromContext(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
		w.WriteHeader(http.StatusOK)

		//localizer := l10n.For(r.Header.Get("Accept-Language"))
		ctx := logging.NewContextWithLogger(r.Context(), log)
		id := r.URL.Query().Get("sensorMeasurementTypes")

		measurements, err := app.GetMeasurementData(ctx, id)
		if err != nil {
			http.Error(w, "could not fetch measurement data", http.StatusBadRequest)
			return
		}

		dataset := components.ChartDataset{
			Data: make([]components.ChartData, 0),
		}

		for _, v := range measurements.Values {
			if v.Value != nil {
				if dataset.Label == "" {
					dataset.Label = v.Unit
				}
				dataset.Data = append(dataset.Data, components.ChartData{X: v.Timestamp.Format(time.DateTime), Y: *v.Value})
			}
		}

		//component := components.LineChart("myChart", dataset)
		component := components.Chart("myChart", "line", dataset)
		component.Render(ctx, w)
	}

	return http.HandlerFunc(fn)
}
