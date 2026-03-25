package sensors

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/application/admin"
	appclient "github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/diwise-web/internal/application/devices"
	"github.com/diwise/diwise-web/internal/application/measurements"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	featuresensors "github.com/diwise/diwise-web/internal/pkg/presentation/webv2/components/features/sensors"
	v2layout "github.com/diwise/diwise-web/internal/pkg/presentation/webv2/components/layout"

	. "github.com/diwise/frontend-toolkit"
)

type sensorDetailsApp interface {
	admin.Management
	devices.Management
	measurements.Management
}

func NewSensorDetailsPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app sensorDetailsApp) http.HandlerFunc {
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
		model, err := composeDetailsModel(ctx, id, app, localizer, editMode)
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

func NewSaveSensorDetailsPage(ctx context.Context, _ LocaleBundle, _ AssetLoaderFunc, app sensorDetailsApp) http.HandlerFunc {
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

func NewMeasurementTypesComponentHandler(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app sensorDetailsApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))
		sensorType := r.URL.Query().Get("sensorType")

		component := featuresensors.MeasurementTypeOptionsField(localizer, featuresensors.MeasurementTypeOptionsProps{
			Options: measurementTypeOptions(localizer, app.GetDeviceProfiles(r.Context()), sensorType, nil, nil),
		})
		helpers.WriteComponentResponse(r.Context(), w, r, component, 8*1024, 0)
	}
}

func NewAttachSensorDialogHandler(_ context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app sensorDetailsApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		renderDialog := func(status int, model featuresensors.AttachSensorDialogViewModel) {
			component := featuresensors.AttachSensorDialog(localizer, assets, model)
			writeComponentStatus(r.Context(), w, status, component)
		}

		switch r.Method {
		case http.MethodGet:
			model, err := composeAttachDialogModel(r.Context(), id, app)
			if err != nil {
				http.Error(w, "could not fetch sensor", http.StatusInternalServerError)
				return
			}
			renderDialog(http.StatusOK, model)
		case http.MethodPost:
			if err := r.ParseForm(); err != nil {
				http.Error(w, "could not parse form data", http.StatusBadRequest)
				return
			}

			model, err := composeAttachDialogModel(r.Context(), id, app)
			if err != nil {
				http.Error(w, "could not fetch sensor", http.StatusInternalServerError)
				return
			}

			sensorID := strings.TrimSpace(r.FormValue("newSensorID"))
			sensorType := strings.TrimSpace(r.FormValue("sensorType"))
			model.SensorID = sensorID
			model.SelectedType = sensorType

			if sensorID == "" {
				model.ErrorMessage = "SensorID kan inte vara tomt"
				renderDialog(http.StatusOK, model)
				return
			}

			if sensorType == "" {
				model.ErrorMessage = "Sensorprofil måste väljas"
				renderDialog(http.StatusOK, model)
				return
			}

			attachCtx := devices.WithAttachSensorID(r.Context(), sensorID)
			if err := app.Attach(attachCtx, id); err != nil {
				model.ErrorMessage = "Kunde inte koppla sensorn"
				switch {
				case errors.Is(err, appclient.ErrNotFound):
					model.ErrorMessage = "Enheten hittades inte"
				case errors.Is(err, appclient.ErrConflict):
					model.ErrorMessage = "SensorID är redan kopplad till en annan enhet"
				}
				renderDialog(http.StatusOK, model)
				return
			}

			if err := app.UpdateSensor(attachCtx, sensorID, map[string]any{
				"sensorID":        sensorID,
				"sensorProfileID": sensorType,
			}); err != nil {
				model.ErrorMessage = "Kunde inte uppdatera sensorprofil"
				switch {
				case errors.Is(err, appclient.ErrNotFound):
					model.ErrorMessage = "Sensorn hittades inte"
				case errors.Is(err, appclient.ErrConflict):
					model.ErrorMessage = "Ogiltig sensorprofil"
				}
				renderDialog(http.StatusOK, model)
				return
			}

			redirectHXOrHTTP(w, r, fmt.Sprintf("/v2/sensors/%s?mode=edit", id))
		default:
			http.Error(w, "", http.StatusBadRequest)
		}
	}
}

func NewDetachSensorDialogHandler(_ context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app sensorDetailsApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		localizer := l10n.For(r.Header.Get("Accept-Language"))

		switch r.Method {
		case http.MethodGet:
			model, err := composeDetachDialogModel(r.Context(), id, app)
			if err != nil {
				http.Error(w, "could not fetch sensor", http.StatusInternalServerError)
				return
			}
			component := featuresensors.DetachSensorDialog(localizer, assets, model)
			helpers.WriteComponentResponse(r.Context(), w, r, component, 8*1024, 0)
		case http.MethodPost:
			model, err := composeDetachDialogModel(r.Context(), id, app)
			if err != nil {
				http.Error(w, "could not fetch sensor", http.StatusInternalServerError)
				return
			}

			if err := app.Deattach(r.Context(), id); err != nil {
				model.ErrorMessage = "Kunde inte koppla bort sensorn"
				component := featuresensors.DetachSensorDialog(localizer, assets, model)
				writeComponentStatus(r.Context(), w, http.StatusOK, component)
				return
			}

			redirectHXOrHTTP(w, r, fmt.Sprintf("/v2/sensors/%s?mode=edit", id))
		default:
			http.Error(w, "", http.StatusBadRequest)
		}
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
	if values := normalizeMeasurementTypeValues(r.Form["measurementType-option[]"]); len(values) > 0 {
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

func normalizeMeasurementTypeValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" || slices.Contains(normalized, part) {
				continue
			}
			normalized = append(normalized, part)
		}
	}

	return normalized
}

func composeDetailsModel(ctx context.Context, id string, app sensorDetailsApp, l10n Localizer, includeEditOptions bool) (featuresensors.SensorDetailsPageViewModel, error) {
	device, err := app.GetDevice(ctx, id)
	if err != nil {
		return featuresensors.SensorDetailsPageViewModel{}, err
	}

	measurements, err := app.GetMeasurementInfo(ctx, id)
	if err != nil {
		return featuresensors.SensorDetailsPageViewModel{}, err
	}

	model := featuresensors.SensorDetailsPageViewModel{
		DeviceID:    device.DeviceID,
		DevEUI:      device.SensorID,
		Name:        device.Name,
		Description: device.Description,
		Tenant:      device.Tenant,
		Active:      device.Active,
		Latitude:    device.Location.Latitude,
		Longitude:   device.Location.Longitude,
		ObservedAt:  device.ObservedAt(),
	}

	if device.Environment != nil {
		model.Environment = *device.Environment
	}

	if device.SensorProfile != nil {
		model.DeviceProfileName = device.SensorProfile.Name
		model.Interval = device.SensorProfile.Interval
	}

	if device.DeviceState != nil {
		model.Online = device.DeviceState.Online
		if model.ObservedAt.IsZero() {
			model.ObservedAt = device.DeviceState.ObservedAt
		}
	}

	if device.SensorStatus != nil {
		model.DeviceStatus = &featuresensors.DeviceStatusViewModel{
			BatteryLevel: device.SensorStatus.BatteryLevel,
			RSSI:         device.SensorStatus.RSSI,
			LoRaSNR:      device.SensorStatus.LoRaSNR,
			Frequency:    device.SensorStatus.Frequency,
			DR:           device.SensorStatus.DR,
			ObservedAt:   device.SensorStatus.ObservedAt,
		}
		if model.ObservedAt.IsZero() {
			model.ObservedAt = device.SensorStatus.ObservedAt
		}
	}

	for _, tp := range device.Types {
		if tp.URN == "" || slices.Contains(model.Types, tp.URN) {
			continue
		}
		model.Types = append(model.Types, tp.URN)
	}

	for _, md := range device.Metadata {
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
		model.TypeOptions = measurementTypeOptions(l10n, app.GetDeviceProfiles(ctx), model.DeviceProfileName, model.Types, sensorTypeLabels(device.Types))
	}

	return model, nil
}

func composeAttachDialogModel(ctx context.Context, id string, app sensorDetailsApp) (featuresensors.AttachSensorDialogViewModel, error) {
	model, err := composeDetailsModel(ctx, id, app, nil, true)
	if err != nil {
		return featuresensors.AttachSensorDialogViewModel{}, err
	}

	return featuresensors.AttachSensorDialogViewModel{
		DeviceID:        model.DeviceID,
		CurrentSensorID: model.DevEUI,
		SensorID:        model.DevEUI,
		SelectedType:    model.DeviceProfileName,
		DeviceProfiles:  model.DeviceProfiles,
	}, nil
}

func composeDetachDialogModel(ctx context.Context, id string, app sensorDetailsApp) (featuresensors.DetachSensorDialogViewModel, error) {
	model, err := composeDetailsModel(ctx, id, app, nil, false)
	if err != nil {
		return featuresensors.DetachSensorDialogViewModel{}, err
	}

	name := model.Name
	if strings.TrimSpace(name) == "" {
		name = model.DeviceID
	}

	return featuresensors.DetachSensorDialogViewModel{
		DeviceID:   model.DeviceID,
		SensorID:   model.DevEUI,
		SensorName: name,
	}, nil
}

func redirectHXOrHTTP(w http.ResponseWriter, r *http.Request, location string) {
	if helpers.IsHxRequest(r) {
		w.Header().Set("HX-Redirect", location)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	http.Redirect(w, r, location, http.StatusFound)
}

func writeComponentStatus(ctx context.Context, w http.ResponseWriter, status int, component templ.Component) {
	var buf bytes.Buffer
	if err := component.Render(ctx, &buf); err != nil {
		http.Error(w, "could not render component", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func deviceProfileOptions(profiles []devices.SensorProfile) []featuresensors.DeviceProfileOption {
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

func measurementTypeOptions(l10n Localizer, profiles []devices.SensorProfile, selectedProfile string, selectedTypes []string, labels map[string]string) []featuresensors.MeasurementTypeOption {
	index := slices.IndexFunc(profiles, func(profile devices.SensorProfile) bool {
		return profile.Name == selectedProfile || profile.Decoder == selectedProfile
	})
	if index < 0 || profiles[index].Types == nil {
		return nil
	}

	options := make([]featuresensors.MeasurementTypeOption, 0, len(*profiles[index].Types))
	for _, value := range *profiles[index].Types {
		label := measurementTypeLabel(value)
		if l10n != nil {
			label = l10n.Get(label)
		}
		if label == "" || label == value {
			if fallback := strings.TrimSpace(labels[value]); fallback != "" {
				label = fallback
			}
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

func sensorTypeLabels(types []devices.Type) map[string]string {
	labels := make(map[string]string, len(types))
	for _, tp := range types {
		if tp.URN == "" || tp.Name == "" {
			continue
		}
		labels[tp.URN] = tp.Name
	}
	return labels
}

func measurementTypeLabel(value string) string {
	parts := strings.Split(value, ":")
	if len(parts) > 1 {
		return strings.Join(parts[1:], "-")
	}

	return value
}
