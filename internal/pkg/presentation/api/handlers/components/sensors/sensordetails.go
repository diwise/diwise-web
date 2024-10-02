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

func NewSensorDetailsPage(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		ctx := helpers.Decorate(r.Context(),
			components.CurrentComponent, "sensors",
		)

		detailsViewModel, err := composeViewModel(ctx, id, app)
		if err != nil {
			http.Error(w, "could not compose view model", http.StatusInternalServerError)
			return
		}

		sensorDetails := components.SensorDetailsPage(localizer, assets, *detailsViewModel)
		page := components.StartPage(version, localizer, assets, sensorDetails)

		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")

		err = page.Render(ctx, w)
		if err != nil {
			http.Error(w, "could not render sensor details page", http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusOK)
	}

	return http.HandlerFunc(fn)
}

func NewSensorDetailsComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		mode := r.URL.Query().Get("mode")
		ctx := helpers.Decorate(r.Context(),
			components.CurrentComponent, "sensors",
		)

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

func composeViewModel(ctx context.Context, id string, app application.DeviceManagement) (*components.SensorDetailsViewModel, error) {
	sensor, err := app.GetSensor(ctx, id)
	if err != nil {
		return nil, err
	}

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

	types := []string{}
	for _, tp := range sensor.Types {
		types = append(types, tp.URN)
	}

	measurements, err := app.GetMeasurementInfo(ctx, id)
	if err != nil {
		return nil, err
	}

	m := make([]string, 0)
	for _, md := range measurements.Values {
		m = append(m, *md.ID)
	}

	mv := make([]components.MeasurementViewModel, 0)
	for _, md := range measurements.Values {
		mvm := components.MeasurementViewModel{
			ID:        *md.ID,
			Timestamp: md.Timestamp,
			Value:     md.Value,
		}
		mv = append(mv, mvm)
	}

	detailsViewModel := components.SensorDetailsViewModel{
		DeviceID:          sensor.DeviceID,
		DevEUI:            sensor.SensorID,
		Name:              sensor.Name,
		Latitude:          sensor.Location.Latitude,
		Longitude:         sensor.Location.Longitude,
		DeviceProfileName: sensor.DeviceProfile.Name,
		Tenant:            sensor.Tenant,
		Description:       sensor.Description,
		Active:            sensor.Active,
		Types:             types,
		Organisations:     tenants,
		DeviceProfiles:    dp,
		MeasurementTypes:  m,
		Measurements:      mv,
		ObservedAt:        sensor.ObservedAt(),
	}
	return &detailsViewModel, nil
}
