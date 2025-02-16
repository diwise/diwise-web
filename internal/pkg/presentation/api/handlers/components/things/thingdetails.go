package things

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"

	//lint:ignore ST1001 it is OK when we do it
	. "github.com/diwise/frontend-toolkit"
)

func NewThingDetailsPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		ctx, thingDetails, err := newThingDetails(r, localizer, assets, app)
		if err != nil {
			http.Error(w, "could not render thing details page", http.StatusInternalServerError)
		}

		thingDetailsPage := components.ThingDetailsPage(localizer, assets, thingDetails)
		page := components.StartPage(version, localizer, assets, thingDetailsPage)

		helpers.WriteComponentResponse(ctx, w, r, page, 1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewThingDetailsComponentHandler(_ context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))
		ctx := r.Context()

		if r.Method == http.MethodDelete {

			id := r.PathValue("id")
			name := r.URL.Query().Get("name")

			if id == "" {
				http.Error(w, "no ID found in url", http.StatusBadRequest)
				return
			}

			if name == "" {
				name = id
			}

			c := components.DeleteThing(localizer, assets, id, name)
			helpers.WriteComponentResponse(ctx, w, r, c, 1024, 0)
			return
		}

		if r.Method == http.MethodPost {
			id := r.PathValue("id")
			if id == "" {
				http.Error(w, "no ID found in url", http.StatusBadRequest)
				return
			}
			err := r.ParseForm()
			if err != nil {
				http.Error(w, "could not parse form", http.StatusBadRequest)
				return
			}

			fields := formToFields(r.Form)
			err = app.UpdateThing(ctx, id, fields)
			if err != nil {
				http.Error(w, "could not update thing", http.StatusBadRequest)
				return
			}
		}

		ctx, thingDetails, err := newThingDetails(r, localizer, assets, app)
		if err != nil {
			http.Error(w, "could not render thing details page", http.StatusInternalServerError)
			return
		}

		helpers.WriteComponentResponse(ctx, w, r, thingDetails, 1024, 0)
	}

	return http.HandlerFunc(fn)
}

func newThingDetails(r *http.Request, localizer Localizer, assets AssetLoaderFunc, app application.ThingManagement) (context.Context, templ.Component, error) {
	ctx := r.Context()

	id := r.PathValue("id")
	if id == "" {
		return ctx, nil, fmt.Errorf("no id found in url")
	}

	editMode := r.URL.Query().Get("mode") == "edit"

	ctx = helpers.Decorate(ctx,
		components.CurrentComponent, "things",
	)

	thing, err := app.GetThing(ctx, id, r.URL.Query())
	if err != nil {
		return ctx, nil, fmt.Errorf("could not compose view model")
	}

	latestValues, err := app.GetLatestValues(ctx, id)
	if err != nil {
		return ctx, nil, fmt.Errorf("could not compose view model")
	}

	thingDetailsViewModel := components.ThingDetailsViewModel{
		Thing: toViewModel(thing),
		Type:  thing.Type,
	}

	for _, lv := range latestValues {
		tabName := strings.ReplaceAll(strings.Replace(lv.ID, id, "", 1)[1:], "/", "-")
		thingDetailsViewModel.Tabs = append(thingDetailsViewModel.Tabs, tabName)

		thingDetailsViewModel.Thing.Latest[tabName] = components.MeasurementViewModel{
			ID:        lv.ID,
			Timestamp: lv.Timestamp,
			Value:     lv.Value,
			Urn:       lv.Urn,
			//StringValue: lv.StringValue,
			BoolValue: lv.BoolValue,
			Unit:      lv.Unit,
		}
	}

	if len(thingDetailsViewModel.Tabs) == 0 {
		thingDetailsViewModel.Tabs = append(thingDetailsViewModel.Tabs, "")
	}

	if thingDetailsViewModel.Thing.SubType != "" {
		thingDetailsViewModel.Type += ":" + thingDetailsViewModel.Thing.SubType
	}

	thingDetailsViewModel.Type = strings.ToLower(thingDetailsViewModel.Type)
	thingDetailsViewModel.Tenant = thingDetailsViewModel.Thing.Tenant

	if editMode {
		validSensors, _ := app.GetValidSensors(ctx, thing.ValidURNs)
		for _, s := range validSensors {
			thingDetailsViewModel.ValidSensors = append(thingDetailsViewModel.ValidSensors, components.ValidSensorViewModel{
				SensorID: s.SensorID,
				DeviceID: s.DeviceID,
				Decoder:  s.Decoder,
			})
		}

		thingDetailsViewModel.Organisations = app.GetTenants(ctx)
		thingDetailsViewModel.Tags, _ = app.GetTags(ctx)

		component := components.EditThingDetails(localizer, assets, thingDetailsViewModel)
		return ctx, component, nil
	}

	return ctx, components.ThingDetails(localizer, assets, thingDetailsViewModel), nil
}

func DeleteThingComponentHandler(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(r.Context(),
			components.CurrentComponent, "things",
		)

		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no ID found in url", http.StatusBadRequest)
			return
		}

		if r.URL.Query().Get("confirmed") == "true" {
			err := app.DeleteThing(ctx, id)
			if err != nil {
				http.Error(w, "could not delete thing", http.StatusInternalServerError)
				return
			}
		}

		http.Redirect(w, r, "/things", http.StatusSeeOther)
	}

	return http.HandlerFunc(fn)
}

func formToFields(form url.Values) map[string]any {
	asFloat := func(s string) (float64, bool) {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f, true
		}
		return 0.0, false
	}

	fields := make(map[string]any)
	fields["tags"] = []string{}

	for k := range form {
		v := form.Get(k)

		if v == "" {
			continue
		}

		switch k {
		case "longitude":
			if _, ok := fields["location"]; !ok {
				fields["location"] = application.Location{}
			}

			if f, ok := asFloat(v); ok {
				loc := fields["location"].(application.Location)
				loc.Longitude = f
				fields["location"] = loc
			}
		case "latitude":
			if _, ok := fields["location"]; !ok {
				fields["location"] = application.Location{}
			}

			if f, ok := asFloat(v); ok {
				loc := fields["location"].(application.Location)
				loc.Latitude = f
				fields["location"] = loc
			}
		case "organisation":
			fields["tenant"] = v
		case "selectedTags":
			fields["tags"] = appendTag(fields["tags"], strings.Split(v, ","))
		case "name":
			fields["name"] = strings.TrimSpace(v)
		case "alternativeName":
			fields["alternativeName"] = strings.TrimSpace(v)
		case "description":
			fields["description"] = strings.TrimSpace(v)
		case "currentDevice":
			refs := strings.Split(v, ",")
			devices := []application.Device{}
			for _, r := range refs {
				exists := slices.ContainsFunc(devices, func(d application.Device) bool {
					return d.DeviceID == strings.TrimSpace(r)
				})
				if !exists {
					devices = append(devices, application.Device{DeviceID: strings.TrimSpace(r)})
				}
			}
			fields["refDevices"] = devices
		case "maxl":
			if f, ok := asFloat(v); ok {
				fields["maxl"] = f
			}
		case "maxd":
			if f, ok := asFloat(v); ok {
				fields["maxd"] = f
			}
		case "angle":
			if f, ok := asFloat(v); ok {
				fields["angle"] = f
			}
		case "offset":
			if f, ok := asFloat(v); ok {
				fields["offset"] = f
			}
		}
	}

	return fields
}

func appendTag(field any, tags []string) []string {
	if field == nil {
		return tags
	}

	switch v := field.(type) {
	case []string:
		return unique(append(tags, v...))
	case string:
		return unique(append(tags, v))
	}

	return tags
}

func unique(s []string) []string {
	unique := make(map[string]struct{})
	for _, v := range s {
		v = strings.TrimSpace(v)
		unique[v] = struct{}{}
	}

	var result []string
	for k := range unique {
		result = append(result, k)
	}

	return result
}
