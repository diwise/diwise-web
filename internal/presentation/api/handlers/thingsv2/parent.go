package thingsv2

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/diwise/diwise-web/internal/application/client"
	appthingsv2 "github.com/diwise/diwise-web/internal/application/thingsv2"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	featuresthingsv2 "github.com/diwise/diwise-web/internal/presentation/web/components/features/thingsv2"

	. "github.com/diwise/frontend-toolkit"
)

func NewThingsV2ParentDialog(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app thingsV2App) http.HandlerFunc {
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
		component := featuresthingsv2.ThingV2ParentDialog(localizer, featuresthingsv2.ThingV2ParentViewModel{
			ThingID:  id,
			Tenant:   tenant,
			Revision: thing.Revision,
			Name:     name,
		})
		helpers.WriteComponentResponse(ctx, w, r, component, 8*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewThingsV2ParentSearch(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app thingsV2App) http.HandlerFunc {
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

		revision, _ := strconv.ParseInt(r.URL.Query().Get("revision"), 10, 64)
		query := strings.TrimSpace(r.URL.Query().Get("query"))

		model := featuresthingsv2.ThingV2ParentResultsViewModel{
			ThingID:  id,
			Tenant:   tenant,
			Revision: revision,
		}
		if query != "" {
			result, err := app.ThingsV2().ListThings(ctx, tenant, appthingsv2.Filter{Name: query, Limit: 20})
			if err != nil {
				http.Error(w, "could not search things", http.StatusInternalServerError)
				return
			}
			for _, thing := range result.Things {
				// Saken kan aldrig bli sin egen förälder.
				if thing.ThingID == id {
					continue
				}
				name := thing.Name
				if name == "" {
					name = thing.ThingID
				}
				model.Results = append(model.Results, featuresthingsv2.ParentCandidate{
					ID:       thing.ThingID,
					Name:     name,
					Category: thing.Category,
				})
			}
		}

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		helpers.WriteComponentResponse(ctx, w, r, featuresthingsv2.ThingV2ParentResults(localizer, model), 8*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewThingsV2SetParent(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app thingsV2App) http.HandlerFunc {
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
		parentID := strings.TrimSpace(r.Form.Get("parentId"))
		revision, _ := strconv.ParseInt(r.Form.Get("revision"), 10, 64)
		if parentID == "" || parentID == id || revision < 1 {
			http.Error(w, "parentId and revision are required", http.StatusBadRequest)
			return
		}

		if _, err := app.ThingsV2().MoveParent(ctx, tenant, id, parentID, revision); err != nil {
			renderParentDialogError(ctx, w, r, l10n, app, tenant, id, err)
			return
		}

		w.Header().Set("HX-Redirect", "/things-v2/"+id+"?tenant="+tenant)
		w.WriteHeader(http.StatusOK)
	}

	return http.HandlerFunc(fn)
}

func NewThingsV2UnlinkParent(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app thingsV2App) http.HandlerFunc {
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
		revision, _ := strconv.ParseInt(r.Form.Get("revision"), 10, 64)
		if revision < 1 {
			http.Error(w, "revision is required", http.StatusBadRequest)
			return
		}

		if _, err := app.ThingsV2().UnlinkParent(ctx, tenant, id, revision); err != nil {
			renderParentDialogError(ctx, w, r, l10n, app, tenant, id, err)
			return
		}

		w.Header().Set("HX-Redirect", "/things-v2/"+id+"?tenant="+tenant)
		w.WriteHeader(http.StatusOK)
	}

	return http.HandlerFunc(fn)
}

func renderParentDialogError(ctx context.Context, w http.ResponseWriter, r *http.Request, l10n LocaleBundle, app thingsV2App, tenant, id string, err error) {
	localizer := l10n.For(r.Header.Get("Accept-Language"))
	message := err.Error()
	if errors.Is(err, client.ErrConflict) {
		message = localizer.Get("saveconflict")
	}

	name := id
	var revision int64
	if thing, thingErr := app.ThingsV2().GetThing(ctx, tenant, id); thingErr == nil {
		if thing.Name != "" {
			name = thing.Name
		}
		revision = thing.Revision
	}

	component := featuresthingsv2.ThingV2ParentDialog(localizer, featuresthingsv2.ThingV2ParentViewModel{
		ThingID:      id,
		Tenant:       tenant,
		Revision:     revision,
		Name:         name,
		ErrorMessage: message,
	})
	helpers.WriteComponentResponse(ctx, w, r, component, 8*1024, 0)
}
