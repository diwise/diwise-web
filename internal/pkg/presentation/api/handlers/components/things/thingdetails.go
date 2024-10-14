package things

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

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

		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		ctx := helpers.Decorate(r.Context(),
			components.CurrentComponent, "things",
		)

		thing, err := app.GetThing(ctx, id)
		if err != nil {
			http.Error(w, "could not compose view model", http.StatusInternalServerError)
			return
		}

		thingDetailsViewModel := components.ThingDetailsViewModel{
			Thing: toViewModel(thing),
		}

		thingDetailsViewModel.Measurements = thingDetailsViewModel.Thing.Measurements

		for _, r := range thing.Related {
			if strings.ToLower(r.Type) != "device" {
				continue
			}

			thingDetailsViewModel.Related = append(thingDetailsViewModel.Related, components.ThingViewModel{
				ThingID: fmt.Sprintf("urn:diwise:%s:%s", r.Type, r.ID),
				ID:      r.ID,
				Type:    r.Type,
			})
		}

		thingDetails := components.ThingDetailsPage(localizer, assets, thingDetailsViewModel)
		page := components.StartPage(version, localizer, assets, thingDetails)

		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")

		err = page.Render(ctx, w)
		if err != nil {
			http.Error(w, "could not render thing details page", http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusOK)
	}

	return http.HandlerFunc(fn)
}

func NewThingDetailsComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		mode := r.URL.Query().Get("mode")

		ctx := helpers.Decorate(r.Context(),
			components.CurrentComponent, "things",
		)

		thing, err := app.GetThing(ctx, id)
		if err != nil {
			http.Error(w, "could not compose view model", http.StatusInternalServerError)
			return
		}

		thingDetailsViewModel := components.ThingDetailsViewModel{
			Thing: toViewModel(thing),
		}

		thingDetailsViewModel.Measurements = thingDetailsViewModel.Thing.Measurements

		for _, r := range thing.Related {
			//TODO: should it be possible to add other types of related things?
			if strings.ToLower(r.Type) != "device" {
				continue
			}

			thingDetailsViewModel.Related = append(thingDetailsViewModel.Related, components.ThingViewModel{
				ThingID: fmt.Sprintf("urn:diwise:%s:%s", r.Type, r.ID),
				ID:      r.ID,
				Type:    r.Type,
			})
		}

		if len(thingDetailsViewModel.Related) > 0 {
			thingDetailsViewModel.RelatedDevice = thingDetailsViewModel.Related[0].ID
		}

		if mode == "edit" {
			urn := []string{}
			switch thing.Type {
			case "combinedsewageoverflow":
				urn = append(urn, "urn:oma:lwm2m:ext:3200")
			case "wastecontainer":
				urn = append(urn, "urn:oma:lwm2m:ext:3300", "urn:oma:lwm2m:ext:3435")
			case "sewer":
				urn = append(urn, "urn:oma:lwm2m:ext:3200")
			case "sewagepumpingstation":
				urn = append(urn, "urn:oma:lwm2m:ext:3200")
			case "passage":
				urn = append(urn, "urn:oma:lwm2m:ext:3200", "urn:oma:lwm2m:ext:3434")
			}

			validSensors, _ := app.GetValidSensors(ctx, urn)
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
			component.Render(ctx, w)
			return
		}

		thingDetails := components.ThingDetails(localizer, assets, thingDetailsViewModel)

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

func NewThingComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		ctx := helpers.Decorate(r.Context(),
			components.CurrentComponent, "things",
		)

		newThingViewModel := components.NewThingViewModel{
			ThingType:     []string{"wastecontainer", "sandstorage", "passage", "combinedsewageoverflow", "room"},
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

func SaveNewThingComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
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

func connectSensor(ctx context.Context, thingID string, fields map[string]any, app application.ThingManagement) error {
	currentID, currentOk := fields["currentDevice"].(string)
	newID, newOk := fields["relatedDevice"].(string)

	if !currentOk || !newOk {
		return fmt.Errorf("could not connect sensor, invalid ID")
	}

	if currentID == newID {
		return nil
	}

	err := app.ConnectSensor(ctx, thingID, currentID, newID)

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
