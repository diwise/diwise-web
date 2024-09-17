package sensors

import (
	"context"
	"net/http"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/locale"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
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
	}
	return &detailsViewModel, nil
}