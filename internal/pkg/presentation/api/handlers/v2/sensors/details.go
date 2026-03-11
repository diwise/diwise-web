package sensors

import (
	"context"
	"net/http"
	"slices"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	featuresensors "github.com/diwise/diwise-web/internal/pkg/presentation/webv2/components/features/sensors"
	v2layout "github.com/diwise/diwise-web/internal/pkg/presentation/webv2/components/layout"

	. "github.com/diwise/frontend-toolkit"
)

func NewSensorDetailsPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "sensors",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		model, err := composeDetailsModel(ctx, id, app)
		if err != nil {
			http.Error(w, "could not fetch sensor", http.StatusInternalServerError)
			return
		}

		content := featuresensors.SensorDetailsPage(localizer, assets, model)
		page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
		if helpers.IsHxRequest(r) {
			page = v2layout.AppShell(localizer, assets, content)
		}

		helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, 0)
	}
}

func composeDetailsModel(ctx context.Context, id string, app application.DeviceManagement) (featuresensors.SensorDetailsPageViewModel, error) {
	sensor, err := app.GetSensor(ctx, id)
	if err != nil {
		return featuresensors.SensorDetailsPageViewModel{}, err
	}

	measurements, err := app.GetMeasurementInfo(ctx, id)
	if err != nil {
		return featuresensors.SensorDetailsPageViewModel{}, err
	}

	model := featuresensors.SensorDetailsPageViewModel{
		DeviceID:    sensor.DeviceID,
		DevEUI:      sensor.SensorID,
		Name:        sensor.Name,
		Description: sensor.Description,
		Tenant:      sensor.Tenant,
		Active:      sensor.Active,
		Latitude:    sensor.Location.Latitude,
		Longitude:   sensor.Location.Longitude,
		ObservedAt:  sensor.ObservedAt(),
	}

	if sensor.Environment != nil {
		model.Environment = *sensor.Environment
	}

	if sensor.DeviceProfile != nil {
		model.DeviceProfileName = sensor.DeviceProfile.Name
	}

	if sensor.DeviceState != nil {
		model.Online = sensor.DeviceState.Online
		if model.ObservedAt.IsZero() {
			model.ObservedAt = sensor.DeviceState.ObservedAt
		}
	}

	if sensor.DeviceStatus != nil {
		model.DeviceStatus = &featuresensors.DeviceStatusViewModel{
			BatteryLevel:    sensor.DeviceStatus.BatteryLevel,
			RSSI:            sensor.DeviceStatus.RSSI,
			LoRaSNR:         sensor.DeviceStatus.LoRaSNR,
			Frequency:       sensor.DeviceStatus.Frequency,
			DR:              sensor.DeviceStatus.DR,
			ObservedAt:      sensor.DeviceStatus.ObservedAt,
		}
		if model.ObservedAt.IsZero() {
			model.ObservedAt = sensor.DeviceStatus.ObservedAt
		}
	}

	for _, tp := range sensor.Types {
		if tp.URN == "" || slices.Contains(model.Types, tp.URN) {
			continue
		}
		model.Types = append(model.Types, tp.URN)
	}

	for _, md := range sensor.Metadata {
		model.Metadata = append(model.Metadata, featuresensors.MetadataViewModel{
			Key:   md.Key,
			Value: md.Value,
		})
	}

	for _, measurement := range measurements {
		item := featuresensors.MeasurementViewModel{
			Timestamp: measurement.Timestamp,
			Unit:      measurement.Unit,
			Value:     measurement.Value,
			BoolValue: measurement.BoolValue,
			String:    measurement.StringValue,
		}
		if measurement.ID != nil {
			item.ID = *measurement.ID
			model.MeasurementTypes = append(model.MeasurementTypes, *measurement.ID)
		}
		if measurement.Name != nil && *measurement.Name != "" {
			item.Name = *measurement.Name
		} else {
			item.Name = item.ID
		}
		model.Measurements = append(model.Measurements, item)
	}

	return model, nil
}
