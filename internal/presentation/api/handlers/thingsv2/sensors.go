package thingsv2

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/diwise/diwise-web/internal/application/client"
	appthingsv2 "github.com/diwise/diwise-web/internal/application/thingsv2"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	featuresthingsv2 "github.com/diwise/diwise-web/internal/presentation/web/components/features/thingsv2"

	. "github.com/diwise/frontend-toolkit"
)

func NewThingsV2SensorsDialog(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app thingsV2App) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(r.Context())

		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		tenant, err := resolveDetailsTenant(r)
		if err != nil {
			http.Error(w, "tenant is required", http.StatusBadRequest)
			return
		}

		model, err := composeSensorsDialog(ctx, app, tenant, id, "")
		if err != nil {
			if errors.Is(err, client.ErrNotFound) {
				http.Error(w, "thing not found", http.StatusNotFound)
				return
			}
			http.Error(w, "could not fetch thing", http.StatusInternalServerError)
			return
		}

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		helpers.WriteComponentResponse(ctx, w, r, featuresthingsv2.ThingV2SensorsDialog(localizer, model), 8*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewThingsV2SensorSearch(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app thingsV2App) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(r.Context())

		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		tenant, err := resolveDetailsTenant(r)
		if err != nil {
			http.Error(w, "tenant is required", http.StatusBadRequest)
			return
		}

		input := strings.TrimSpace(r.URL.Query().Get("input"))
		revision, _ := strconv.ParseInt(r.URL.Query().Get("revision"), 10, 64)
		query := strings.TrimSpace(r.URL.Query().Get("query"))

		model := featuresthingsv2.ThingV2SensorResultsViewModel{
			ThingID:  id,
			Tenant:   tenant,
			Revision: revision,
			Input:    input,
		}
		if input != "" && query != "" {
			spec, err := templateSpecForThing(ctx, app, tenant, id)
			if err != nil {
				http.Error(w, "could not fetch template", http.StatusInternalServerError)
				return
			}
			urns := signalURNs(spec, input)
			sensors, err := app.GetValidSensors(ctx, urns, query)
			if err != nil {
				http.Error(w, "could not search sensors", http.StatusInternalServerError)
				return
			}
			for _, sensor := range sensors {
				name := strings.TrimSpace(sensor.Name)
				if name == "" {
					name = sensor.DeviceID
				}
				model.Results = append(model.Results, featuresthingsv2.SensorCandidateViewModel{
					DeviceID: sensor.DeviceID,
					Name:     name,
				})
			}
		}

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		helpers.WriteComponentResponse(ctx, w, r, featuresthingsv2.ThingV2SensorResults(localizer, model), 8*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewThingsV2SetSensor(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app thingsV2App) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(r.Context())

		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		tenant, err := resolveDetailsTenant(r)
		if err != nil {
			http.Error(w, "tenant is required", http.StatusBadRequest)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "could not parse form data", http.StatusBadRequest)
			return
		}
		input := strings.TrimSpace(r.Form.Get("input"))
		deviceID := strings.TrimSpace(r.Form.Get("deviceId"))
		revision, _ := strconv.ParseInt(r.Form.Get("revision"), 10, 64)
		if input == "" || deviceID == "" || revision < 1 {
			http.Error(w, "input, deviceId and revision are required", http.StatusBadRequest)
			return
		}

		if err := swapThingSensor(ctx, app, tenant, id, revision, input, deviceID); err != nil {
			renderSensorsDialogError(ctx, w, r, l10n, app, tenant, id, err)
			return
		}

		w.Header().Set("HX-Redirect", "/things-v2/"+id+"?tenant="+tenant)
		w.WriteHeader(http.StatusOK)
	}

	return http.HandlerFunc(fn)
}

func NewThingsV2UnbindSensor(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app thingsV2App) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(r.Context())

		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		tenant, err := resolveDetailsTenant(r)
		if err != nil {
			http.Error(w, "tenant is required", http.StatusBadRequest)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "could not parse form data", http.StatusBadRequest)
			return
		}
		input := strings.TrimSpace(r.Form.Get("input"))
		revision, _ := strconv.ParseInt(r.Form.Get("revision"), 10, 64)
		if input == "" || revision < 1 {
			http.Error(w, "input and revision are required", http.StatusBadRequest)
			return
		}

		if err := swapThingSensor(ctx, app, tenant, id, revision, input, ""); err != nil {
			renderSensorsDialogError(ctx, w, r, l10n, app, tenant, id, err)
			return
		}

		w.Header().Set("HX-Redirect", "/things-v2/"+id+"?tenant="+tenant)
		w.WriteHeader(http.StatusOK)
	}

	return http.HandlerFunc(fn)
}

// swapThingSensor sätter (tom deviceID = tar bort) ingångens koppling och
// behåller övriga. channel sätts aldrig (tom = alla kanaler).
func swapThingSensor(ctx context.Context, app thingsV2App, tenant, id string, revision int64, input, deviceID string) error {
	spec, err := templateSpecForThing(ctx, app, tenant, id)
	if err != nil {
		return err
	}
	def, ok := spec.Template.PropertyDefs[input]
	if !ok || len(def.Signals) == 0 {
		return errors.New("input is not bindable")
	}

	bindings, err := app.ThingsV2().GetBindings(ctx, tenant, id)
	if err != nil {
		return err
	}
	swapped := make([]appthingsv2.Binding, 0, len(bindings)+1)
	for _, binding := range bindings {
		if binding.Input == input {
			continue
		}
		swapped = append(swapped, binding)
	}
	if deviceID != "" {
		swapped = append(swapped, appthingsv2.Binding{
			DeviceID: deviceID,
			Object:   def.Signals[0].Object,
			Resource: def.Signals[0].Resource,
			Input:    input,
		})
	}

	_, err = app.ThingsV2().SwapBindings(ctx, tenant, id, swapped, revision)
	return err
}

func templateSpecForThing(ctx context.Context, app thingsV2App, tenant, id string) (appthingsv2.TemplateSpec, error) {
	thing, err := app.ThingsV2().GetThing(ctx, tenant, id)
	if err != nil {
		return appthingsv2.TemplateSpec{}, err
	}
	templates, err := app.ThingsV2().ListTemplates(ctx, "", "")
	if err != nil {
		return appthingsv2.TemplateSpec{}, err
	}
	spec, found := findTemplateSpec(templates, thing.TemplateID, thing.TemplateVersion)
	if !found {
		return appthingsv2.TemplateSpec{}, errors.New("unknown template")
	}
	return spec, nil
}

func signalURNs(spec appthingsv2.TemplateSpec, input string) []string {
	def, ok := spec.Template.PropertyDefs[input]
	if !ok {
		return nil
	}
	urns := make([]string, 0, len(def.Signals))
	for _, signal := range def.Signals {
		if signal.Object != "" {
			urns = append(urns, signal.Object)
		}
	}
	return urns
}

func composeSensorsDialog(ctx context.Context, app thingsV2App, tenant, id, errorMessage string) (featuresthingsv2.ThingV2SensorsViewModel, error) {
	thing, err := app.ThingsV2().GetThing(ctx, tenant, id)
	if err != nil {
		return featuresthingsv2.ThingV2SensorsViewModel{}, err
	}
	spec, err := templateSpecForThing(ctx, app, tenant, id)
	if err != nil {
		return featuresthingsv2.ThingV2SensorsViewModel{}, err
	}
	bindings, err := app.ThingsV2().GetBindings(ctx, tenant, id)
	if err != nil {
		bindings = nil
	}

	byInput := map[string]appthingsv2.Binding{}
	for _, binding := range bindings {
		if _, exists := byInput[binding.Input]; !exists {
			byInput[binding.Input] = binding
		}
	}

	name := thing.Name
	if name == "" {
		name = thing.ThingID
	}
	model := featuresthingsv2.ThingV2SensorsViewModel{
		ThingID:      id,
		Tenant:       tenant,
		Revision:     thing.Revision,
		Name:         name,
		ErrorMessage: errorMessage,
	}
	for _, input := range bindableInputs(spec) {
		item := featuresthingsv2.SensorBindingViewModel{
			Input:    input,
			Label:    inputLabel(spec, thing, input),
			Object:   firstSignalObject(spec, input),
			Resource: firstSignalResource(spec, input),
		}
		if binding, ok := byInput[input]; ok {
			item.DeviceID = binding.DeviceID
			item.DeviceName = binding.DeviceID
			if device, err := app.GetDevice(ctx, binding.DeviceID); err == nil && device.Name != "" {
				item.DeviceName = device.Name
			}
		}
		model.Bindings = append(model.Bindings, item)
	}

	return model, nil
}

// bindableInputs är mallens egenskaper med minst en signal, sorterade.
func bindableInputs(spec appthingsv2.TemplateSpec) []string {
	inputs := make([]string, 0, len(spec.Template.PropertyDefs))
	for id, def := range spec.Template.PropertyDefs {
		if len(def.Signals) > 0 {
			inputs = append(inputs, id)
		}
	}
	sort.Strings(inputs)
	return inputs
}

func inputLabel(spec appthingsv2.TemplateSpec, thing appthingsv2.Thing, input string) string {
	if def, ok := spec.Template.PropertyDefs[input]; ok && def.DisplayName != "" {
		return def.DisplayName
	}
	if value, ok := thing.Values[input]; ok && value.DisplayName != "" {
		return value.DisplayName
	}
	return input
}

func firstSignalObject(spec appthingsv2.TemplateSpec, input string) string {
	if def, ok := spec.Template.PropertyDefs[input]; ok && len(def.Signals) > 0 {
		return def.Signals[0].Object
	}
	return ""
}

func firstSignalResource(spec appthingsv2.TemplateSpec, input string) string {
	if def, ok := spec.Template.PropertyDefs[input]; ok && len(def.Signals) > 0 {
		return def.Signals[0].Resource
	}
	return ""
}

func renderSensorsDialogError(ctx context.Context, w http.ResponseWriter, r *http.Request, l10n LocaleBundle, app thingsV2App, tenant, id string, err error) {
	localizer := l10n.For(r.Header.Get("Accept-Language"))
	message := err.Error()
	if errors.Is(err, client.ErrConflict) {
		message = localizer.Get("saveconflict")
	}

	model, modelErr := composeSensorsDialog(ctx, app, tenant, id, message)
	if modelErr != nil {
		http.Error(w, "could not fetch thing", http.StatusInternalServerError)
		return
	}
	helpers.WriteComponentResponse(ctx, w, r, featuresthingsv2.ThingV2SensorsDialog(localizer, model), 8*1024, 0)
}
