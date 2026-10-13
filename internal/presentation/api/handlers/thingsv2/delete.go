package thingsv2

import (
	"context"
	"errors"
	"net/http"

	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	featuresthingsv2 "github.com/diwise/diwise-web/internal/presentation/web/components/features/thingsv2"

	. "github.com/diwise/frontend-toolkit"
)

func NewThingsV2DeleteDialog(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app thingsV2App) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(r.Context())

		id := r.PathValue("id")
		if !requireThingID(w, id) {
			return
		}

		tenant, err := resolveDetailsTenant(r)
		if err != nil {
			http.Error(w, "tenant is required", http.StatusBadRequest)
			return
		}

		thing, err := app.ThingsV2().GetThing(ctx, tenant, id)
		if err != nil {
			if errors.Is(err, client.ErrNotFound) {
				http.Error(w, "thing not found", http.StatusNotFound)
				return
			}
			http.Error(w, "could not fetch thing", http.StatusInternalServerError)
			return
		}

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		name := thing.Name
		if name == "" {
			name = thing.ThingID
		}
		component := featuresthingsv2.ThingV2DeleteDialog(localizer, featuresthingsv2.ThingV2DeleteViewModel{
			ThingID: id,
			Tenant:  tenant,
			Name:    name,
		})
		helpers.WriteComponentResponse(ctx, w, r, component, 8*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewThingsV2DeletePage(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app thingsV2App) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(r.Context())

		id := r.PathValue("id")
		if !requireThingID(w, id) {
			return
		}

		tenant, err := resolveDetailsTenant(r)
		if err != nil {
			http.Error(w, "tenant is required", http.StatusBadRequest)
			return
		}

		if err := app.ThingsV2().DeleteThing(ctx, tenant, id); err != nil {
			localizer := l10n.For(r.Header.Get("Accept-Language"))
			message := err.Error()
			if errors.Is(err, client.ErrConflict) {
				message = localizer.Get("deleteblocked")
			}
			name := id
			if thing, thingErr := app.ThingsV2().GetThing(ctx, tenant, id); thingErr == nil && thing.Name != "" {
				name = thing.Name
			}
			component := featuresthingsv2.ThingV2DeleteDialog(localizer, featuresthingsv2.ThingV2DeleteViewModel{
				ThingID:      id,
				Tenant:       tenant,
				Name:         name,
				ErrorMessage: message,
			})
			helpers.WriteComponentResponse(ctx, w, r, component, 8*1024, 0)
			return
		}

		w.Header().Set("HX-Redirect", "/things-v2")
		w.WriteHeader(http.StatusOK)
	}

	return http.HandlerFunc(fn)
}
