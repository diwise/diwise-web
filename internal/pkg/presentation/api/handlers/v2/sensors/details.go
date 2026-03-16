package sensors

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

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
		editMode := r.URL.Query().Get("mode") == "edit"
		model, err := composeDetailsModel(ctx, id, app, editMode)
		if err != nil {
			http.Error(w, "could not fetch sensor", http.StatusInternalServerError)
			return
		}

		content := featuresensors.SensorDetailsPage(localizer, assets, model)
		if editMode {
			content = featuresensors.EditSensorDetailsPage(localizer, assets, model)
		}
		page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
		if helpers.IsHxRequest(r) {
			page = v2layout.AppShell(localizer, assets, content)
		}

		helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, 0)
	}
}

func NewSaveSensorDetailsPage(ctx context.Context, _ LocaleBundle, _ AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "could not parse form data", http.StatusBadRequest)
			return
		}

		if err := app.UpdateSensor(r.Context(), id, buildSensorUpdateFields(r)); err != nil {
			http.Error(w, "could not update sensor", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/v2/sensors/%s", id), http.StatusFound)
	}
}

func NewMeasurementTypesComponentHandler(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))
		sensorType := r.URL.Query().Get("sensorType")

		component := featuresensors.MeasurementTypeOptionsField(localizer, featuresensors.MeasurementTypeOptionsProps{
			Options: measurementTypeOptions(localizer, app.GetDeviceProfiles(r.Context()), sensorType, nil),
		})
		helpers.WriteComponentResponse(r.Context(), w, r, component, 8*1024, 0)
	}
}

func buildSensorUpdateFields(r *http.Request) map[string]any {
	fields := map[string]any{
		"deviceID": r.Form.Get("id"),
		"active":   r.Form.Get("active") == "on",
	}

	if value := strings.TrimSpace(r.Form.Get("name")); value != "" {
		fields["name"] = value
	}
	if value := strings.TrimSpace(r.Form.Get("description")); value != "" {
		fields["description"] = value
	}
	if value := strings.TrimSpace(r.Form.Get("sensorType")); value != "" {
		fields["deviceProfile"] = value
	}
	if value := strings.TrimSpace(r.Form.Get("organisation")); value != "" {
		fields["tenant"] = value
	}
	if value := strings.TrimSpace(r.Form.Get("environment")); value != "" {
		fields["environment"] = value
	}
	if value := strings.TrimSpace(r.Form.Get("interval")); value != "" {
		fields["interval"] = value
	}
	if values := r.Form["measurementType-option[]"]; len(values) > 0 {
		fields["types"] = values
	}

	for formKey, fieldKey := range map[string]string{
		"latitude":  "latitude",
		"longitude": "longitude",
	} {
		if value := strings.TrimSpace(r.Form.Get(formKey)); value != "" {
			if parsed, err := strconv.ParseFloat(value, 64); err == nil {
				fields[fieldKey] = parsed
			}
		}
	}

	return fields
}

func composeDetailsModel(ctx context.Context, id string, app application.DeviceManagement, includeEditOptions bool) (featuresensors.SensorDetailsPageViewModel, error) {
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
		model.Interval = sensor.DeviceProfile.Interval
	}

	if sensor.DeviceState != nil {
		model.Online = sensor.DeviceState.Online
		if model.ObservedAt.IsZero() {
			model.ObservedAt = sensor.DeviceState.ObservedAt
		}
	}

	if sensor.DeviceStatus != nil {
		model.DeviceStatus = &featuresensors.DeviceStatusViewModel{
			BatteryLevel: sensor.DeviceStatus.BatteryLevel,
			RSSI:         sensor.DeviceStatus.RSSI,
			LoRaSNR:      sensor.DeviceStatus.LoRaSNR,
			Frequency:    sensor.DeviceStatus.Frequency,
			DR:           sensor.DeviceStatus.DR,
			ObservedAt:   sensor.DeviceStatus.ObservedAt,
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

	if includeEditOptions {
		model.Organisations = app.GetTenants(ctx)
		model.DeviceProfiles = deviceProfileOptions(app.GetDeviceProfiles(ctx))
		model.TypeOptions = measurementTypeOptions(nil, app.GetDeviceProfiles(ctx), model.DeviceProfileName, model.Types)
	}

	return model, nil
}

func deviceProfileOptions(profiles []application.DeviceProfile) []featuresensors.DeviceProfileOption {
	options := make([]featuresensors.DeviceProfileOption, 0, len(profiles))
	for _, profile := range profiles {
		option := featuresensors.DeviceProfileOption{
			Name:     profile.Name,
			Decoder:  profile.Decoder,
			Interval: profile.Interval,
		}
		if profile.Types != nil {
			option.Types = append(option.Types, (*profile.Types)...)
		}
		options = append(options, option)
	}
	return options
}

func measurementTypeOptions(l10n Localizer, profiles []application.DeviceProfile, selectedProfile string, selectedTypes []string) []featuresensors.MeasurementTypeOption {
	index := slices.IndexFunc(profiles, func(profile application.DeviceProfile) bool {
		return profile.Name == selectedProfile || profile.Decoder == selectedProfile
	})
	if index < 0 || profiles[index].Types == nil {
		return nil
	}

	options := make([]featuresensors.MeasurementTypeOption, 0, len(*profiles[index].Types))
	for _, value := range *profiles[index].Types {
		label := value
		parts := strings.Split(value, ":")
		if len(parts) > 1 {
			label = strings.Join(parts[1:], "-")
		}
		if l10n != nil {
			label = l10n.Get(label)
		}

		options = append(options, featuresensors.MeasurementTypeOption{
			Value:    value,
			Label:    label,
			Selected: slices.Contains(selectedTypes, value),
		})
	}

	slices.SortFunc(options, func(a, b featuresensors.MeasurementTypeOption) int {
		return strings.Compare(a.Label, b.Label)
	})

	return options
}
