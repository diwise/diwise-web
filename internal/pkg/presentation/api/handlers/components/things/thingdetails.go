package things

import (
	"context"
	"fmt"
	"net/http"
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

		if mode == "edit" {
			tenants := app.GetTenants(ctx)

			thingDetailsViewModel.Organisations = tenants

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
				case "longitude":
					if f, ok := asFloat(v); ok {
						fields[k] = f
					}
				case "latitude":
					if f, ok := asFloat(v); ok {
						fields[k] = f
					}
				case "organisation":
					fields["tenant"] = v
				default:
					fields[k] = r.Form.Get(k)
				}
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
