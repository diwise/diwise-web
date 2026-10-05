package thingsv2

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/diwise/diwise-web/internal/application/client"
	featuresthings "github.com/diwise/diwise-web/internal/presentation/web/components/features/things"
	view "github.com/diwise/diwise-web/internal/presentation/web/components/features/thingsv2"
)

type formStatusWriter struct {
	http.ResponseWriter
	status int
}

func (w formStatusWriter) WriteHeader(status int) {
	if status == http.StatusOK {
		status = w.status
	}
	w.ResponseWriter.WriteHeader(status)
}
func formErrorStatus(err error) int {
	if errors.Is(err, client.ErrUnauthorized) {
		return http.StatusUnauthorized
	}
	if errors.Is(err, client.ErrForbidden) {
		return http.StatusForbidden
	}
	return http.StatusOK
}

// Backend failures must never require successful refetches to echo input.
// Fallback models render every submitted parameter and relation, even when
// the catalog/config is temporarily unavailable or permissions changed.
func fallbackCreate(r *http.Request, message string) view.ThingV2CreateViewModel {
	model := view.ThingV2CreateViewModel{
		Tenants: tokenTenants(r), Tenant: r.Form.Get("tenant"), Template: r.Form.Get("template"),
		TemplateDisplay: r.Form.Get("template"), Variant: r.Form.Get("variant"),
		Name: r.Form.Get("name"), Description: r.Form.Get("description"),
		Latitude: r.Form.Get("latitude"), Longitude: r.Form.Get("longitude"),
		GeometryMode: r.Form.Get("geometryMode"), GeometryKinds: []string{"Point", "Polygon"},
		GeometryJSON: r.Form.Get("geometry"), AllowNoLocation: true, ErrorMessage: message,
	}
	model.Templates = []featuresthings.TypeOption{{Value: model.Template, Label: model.Template}}
	if model.Variant != "" {
		model.Variants = []featuresthings.TypeOption{{Value: model.Variant, Label: model.Variant}}
	}
	keys := make([]string, 0, len(r.Form))
	for key := range r.Form {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if name, ok := strings.CutPrefix(key, "param."); ok {
			model.Params = append(model.Params, view.ParamFieldViewModel{Name: name, Label: name, Value: r.Form.Get(key)})
		}
		if name, ok := strings.CutPrefix(key, "relation."); ok {
			slot := view.RelationSlotViewModel{Name: name}
			for _, id := range r.Form[key] {
				slot.Options = append(slot.Options, view.RelationOption{ID: id, Label: id, Selected: true})
			}
			model.Relations = append(model.Relations, slot)
		}
	}
	return model
}

func fallbackEdit(r *http.Request, id, message string) view.ThingV2EditViewModel {
	create := fallbackCreate(r, message)
	revision, _ := strconv.ParseInt(r.Form.Get("revision"), 10, 64)
	return view.ThingV2EditViewModel{
		ThingID: id, Tenant: create.Tenant, Revision: revision,
		TemplateDisplay: create.TemplateDisplay, Variant: create.Variant, Variants: create.Variants,
		Params: create.Params, Relations: create.Relations, Name: create.Name, Description: create.Description,
		Latitude: create.Latitude, Longitude: create.Longitude, GeometryMode: create.GeometryMode,
		GeometryKinds: create.GeometryKinds, GeometryJSON: create.GeometryJSON, AllowNoLocation: true,
		ErrorMessage: message,
	}
}
