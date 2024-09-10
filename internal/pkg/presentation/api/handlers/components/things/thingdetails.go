package things

import (
	"context"
	"fmt"
	"net/http"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/locale"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
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

		detailsViewModel := components.ThingDetailsViewModel{
			Thing: components.ThingViewModel{
				ThingID:   thing.ThingID,
				Latitude:  thing.Location.Latitude,
				Longitude: thing.Location.Longitude,
				Tenant:    thing.Tenant,
				Type:      thing.Type,
			},
		}

		for _, r := range thing.Related {
			detailsViewModel.Related = append(detailsViewModel.Related, components.ThingViewModel{
				ThingID: fmt.Sprintf("urn:diwise:%s:%s", r.Type, r.ID),
				ID:      r.ID,
				Type:    r.Type,
			})
		}

		thingDetails := components.ThingDetailsPage(localizer, assets, detailsViewModel)
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
	//version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		//mode := r.URL.Query().Get("mode")
		ctx := helpers.Decorate(r.Context(),
			components.CurrentComponent, "things",
		)

		thing, err := app.GetThing(ctx, id)
		if err != nil {
			http.Error(w, "could not compose view model", http.StatusInternalServerError)
			return
		}

		thingsViewModel := components.ThingDetailsViewModel{
			Thing: components.ThingViewModel{
				ThingID:   thing.ThingID,
				Latitude:  thing.Location.Latitude,
				Longitude: thing.Location.Longitude,
				Tenant:    thing.Tenant,
				Type:      thing.Type,
			},
		}

		for _, r := range thing.Related {
			thingsViewModel.Related = append(thingsViewModel.Related, components.ThingViewModel{
				ThingID: fmt.Sprintf("urn:diwise:%s:%s", r.Type, r.ID),
				ID:      r.ID,
				Type:    r.Type,
			})
		}

		thingDetails := components.ThingDetails(localizer, assets, thingsViewModel)
		//page := components.StartPage(version, localizer, assets, thingDetails)

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
