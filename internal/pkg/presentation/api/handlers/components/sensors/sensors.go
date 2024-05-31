package sensors

import (
	"context"
	"net/http"
	"strconv"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/locale"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

func NewSensorDetailsComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.SensorService) http.HandlerFunc {
	log := logging.GetFromContext(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "no id found i url", http.StatusBadRequest)
			return
		}

		mode := r.URL.Query().Get("mode")
		ctx := r.Context()

		sensor, err := app.GetSensor(ctx, id)
		if err != nil {
			log.Error("unable to get sensor details", "err", err.Error())
			http.Error(w, "unable to get sensor details", http.StatusInternalServerError)
			return
		}

		detailsViewModel := components.SensorDetailsViewModel{
			DeviceID:          sensor.DeviceID,
			Name:              sensor.Name,
			Latitude:          sensor.Location.Latitude,
			Longitude:         sensor.Location.Longitude,
			DeviceProfileName: sensor.DeviceProfile.Name,
			Tenant:            sensor.Tenant,
			Description:       sensor.Description,
			Active:            sensor.Active,
		}

		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
		w.WriteHeader(http.StatusOK)

		if mode == "edit" {
			component := components.EditSensorDetails(localizer, assets, detailsViewModel)
			component.Render(ctx, w)
			return
		}

		component := components.SensorDetails(localizer, assets, detailsViewModel)
		component.Render(ctx, w)
	}

	return http.HandlerFunc(fn)
}

func NewSaveSensorDetailsComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.SensorService) http.HandlerFunc {
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

		asFloat := func(s string) float64 {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				return f
			}
			return 0.0
		}

		id := r.Form.Get("id")
		active := r.Form.Get("active")
		name := r.Form.Get("name")
		longitude := r.Form.Get("longitude")
		latitude := r.Form.Get("latitude")
		//sensorType := r.Form.Get("sensorType")
		//measurementType := r.Form["measurementType"]
		organisation := r.Form.Get("organisation")
		description := r.Form.Get("description")

		if r.Form.Has("save") {
			sensor := application.Sensor{
				DeviceID: id,
				Active: asBool(active),
				Name:   name,
				Tenant: organisation,
				Location: application.Location{
					Latitude:  asFloat(latitude),
					Longitude: asFloat(longitude),
				},
				Description: description,
			}

			err = app.UpdateSensor(ctx, sensor)
			if err != nil {
				http.Error(w, "could not update sensor", http.StatusInternalServerError)
				return
			}
		}

		http.Redirect(w, r, "/sensors/"+id, http.StatusFound)
	}

	return http.HandlerFunc(fn)
}

func NewTableSensorsComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.SensorService) http.HandlerFunc {
	log := logging.GetFromContext(ctx)
	
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
		w.WriteHeader(http.StatusOK)

		localizer := l10n.For(r.Header.Get("Accept-Language"))

		page := helpers.UrlParamOrDefault(r, "page", "1")
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

		ctx = helpers.Decorate(
			ctx,
			components.PageIndex, page,
			components.PageLast, sensorResult.TotalRecords/limit,
			components.PageSize, limit,
		)

		component := components.SensorTable(localizer, assets, listViewModel)
		component.Render(ctx, w)
	}

	return http.HandlerFunc(fn)
}
