package things

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/locale"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

func NewThingDetailsPage(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		thingDetails, err := newThingDetails(r, localizer, assets, app)
		if err != nil {
			http.Error(w, "could not render thing details page", http.StatusInternalServerError)
		}

		page := components.StartPage(version, localizer, assets, thingDetails)

		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")

		err = page.Render(ctx, w)
		if err != nil {
			http.Error(w, "could not render thing details page", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}

	return http.HandlerFunc(fn)
}

func NewThingDetailsComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		thingDetails, err := newThingDetails(r, localizer, assets, app)
		if err != nil {
			http.Error(w, "could not render thing details page", http.StatusInternalServerError)
			return
		}

		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")

		err = thingDetails.Render(ctx, w)
		if err != nil {
			http.Error(w, "could not render thing details page", http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusOK)
	}

	return http.HandlerFunc(fn)
}

func newThingDetails(r *http.Request, localizer locale.Localizer, assets assets.AssetLoaderFunc, app application.ThingManagement) (templ.Component, error) {
	id := r.PathValue("id")
	if id == "" {
		return nil, fmt.Errorf("no id found in url")
	}

	editMode := r.URL.Query().Get("mode") == "edit"

	ctx := helpers.Decorate(r.Context(),
		components.CurrentComponent, "things",
	)
	thing, err := app.GetThing(ctx, id, r.URL.Query())
	if err != nil {
		return nil, fmt.Errorf("could not compose view model")
	}

	thingDetailsViewModel := components.ThingDetailsViewModel{
		Thing: toViewModel(thing),
	}

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
		return component, nil
	}

	return components.ThingDetails(localizer, assets, thingDetailsViewModel), nil
}

func NewThingComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		ctx := helpers.Decorate(r.Context(),
			components.CurrentComponent, "things",
		)

		thingTypes, _ := app.GetTypes(ctx)

		newThingViewModel := components.NewThingViewModel{
			ThingType:     thingTypes,
			Organisations: app.GetTenants(ctx),
		}

		newThingViewModel.Tags, _ = app.GetTags(ctx)

		component := components.NewThing(localizer, assets, newThingViewModel)

		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")

		err := component.Render(ctx, w)
		if err != nil {
			http.Error(w, "could not render new thing page", http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusOK)
	}

	return http.HandlerFunc(fn)
}

func NewSaveThingDetailsComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
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

		id := r.Form.Get("id")

		if r.Form.Has("save") {
			fields := formToFields(r.Form)
			err = connectSensor(ctx, id, fields, app)
			if err != nil {
				http.Error(w, "could not connect sensor", http.StatusInternalServerError)
				return
			}
			err = app.UpdateThing(ctx, id, fields)
			if err != nil {
				http.Error(w, "could not update thing", http.StatusInternalServerError)
				return
			}
		}

		http.Redirect(w, r, "/things/"+id, http.StatusFound)
	}

	return http.HandlerFunc(fn)
}

func DeleteThingComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		ctx := helpers.Decorate(r.Context(),
			components.CurrentComponent, "things",
		)

		component := components.DeleteThing(localizer, assets)

		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")

		err := component.Render(ctx, w)
		if err != nil {
			http.Error(w, "could not render delete thing", http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusOK)
	}

	return http.HandlerFunc(fn)
}

func connectSensor(ctx context.Context, thingID string, fields map[string]any, app application.ThingManagement) error {
	currentID, currentOk := fields["currentDevice"].(string)
	newID, newOk := fields["relatedDevice"].(string)

	if !currentOk || !newOk {
		return fmt.Errorf("could not connect sensor, invalid ID")
	}

	if currentID == newID {
		return nil
	}

	err := app.ConnectSensor(ctx, thingID, []string{})

	delete(fields, "currentDevice")
	delete(fields, "relatedDevice")

	return err
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
	fields["currentDevice"] = ""

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
		case "description":
			fields["description"] = strings.TrimSpace(v)
		case "currentDevice":
			fields["currentDevice"] = strings.TrimSpace(v)
		case "relatedDevice":
			fields["relatedDevice"] = strings.TrimSpace(v)
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
