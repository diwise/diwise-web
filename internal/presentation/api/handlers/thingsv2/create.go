package thingsv2

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	appthingsv2 "github.com/diwise/diwise-web/internal/application/thingsv2"
	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	featuresthings "github.com/diwise/diwise-web/internal/presentation/web/components/features/things"
	featuresthingsv2 "github.com/diwise/diwise-web/internal/presentation/web/components/features/thingsv2"
	v2layout "github.com/diwise/diwise-web/internal/presentation/web/components/layout"
	"github.com/google/uuid"

	. "github.com/diwise/frontend-toolkit"
)

func NewThingsV2NewPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app thingsV2App) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "things-v2",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		model, err := composeCreateModel(ctx, r, app, r.URL.Query().Get("tenant"), r.URL.Query().Get("template"), nil, "")
		if err != nil {
			http.Error(w, "could not fetch templates", http.StatusInternalServerError)
			return
		}

		content := featuresthingsv2.ThingV2CreatePage(localizer, model)
		page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
		if helpers.IsHxRequest(r) {
			page = v2layout.AppShell(localizer, assets, content)
		}

		helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewThingsV2CreatePage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app thingsV2App) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "things-v2",
		)

		if err := r.ParseForm(); err != nil {
			http.Error(w, "could not parse form data", http.StatusBadRequest)
			return
		}

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		spec, tenant, fail := buildCreateSpec(ctx, app, r)
		if fail != "" {
			model, err := composeCreateModel(ctx, r, app, r.Form.Get("tenant"), r.Form.Get("template"), r.Form, fail)
			if err != nil {
				http.Error(w, "could not fetch templates", http.StatusInternalServerError)
				return
			}
			content := featuresthingsv2.ThingV2CreatePage(localizer, model)
			page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
			if helpers.IsHxRequest(r) {
				page = v2layout.AppShell(localizer, assets, content)
			}
			helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, 0)
			return
		}

		thing, err := app.ThingsV2().CreateThing(ctx, tenant, spec)
		if err != nil {
			model, modelErr := composeCreateModel(ctx, r, app, r.Form.Get("tenant"), r.Form.Get("template"), r.Form, err.Error())
			if modelErr != nil {
				http.Error(w, "could not create thing", http.StatusInternalServerError)
				return
			}
			content := featuresthingsv2.ThingV2CreatePage(localizer, model)
			page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
			if helpers.IsHxRequest(r) {
				page = v2layout.AppShell(localizer, assets, content)
			}
			helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, 0)
			return
		}

		target := fmt.Sprintf("/things-v2/%s?tenant=%s", thing.ThingID, tenant)
		if helpers.IsHxRequest(r) {
			w.Header().Set("HX-Redirect", target)
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, target, http.StatusFound)
	}

	return http.HandlerFunc(fn)
}

func tokenTenants(r *http.Request) []string {
	// Routen är redan skyddad med rätt scope; här räcker tokenens tenants.
	tenants := auth.GetTenantsWithAllowedScopes(r.Context(), auth.AnyScope)
	unique := tenants[:0]
	for _, tenant := range tenants {
		tenant = strings.TrimSpace(tenant)
		if tenant == "" || slices.Contains(unique, tenant) {
			continue
		}
		unique = append(unique, tenant)
	}
	return unique
}

func resolveCreateTenant(r *http.Request, tenants []string, wanted string) (string, bool) {
	if wanted = strings.TrimSpace(wanted); wanted != "" {
		// Servern auktoriserar per tenant; här räcker det att välja.
		if len(tenants) == 0 || slices.Contains(tenants, wanted) {
			return wanted, true
		}
		return "", false
	}
	if len(tenants) == 1 {
		return tenants[0], true
	}
	return "", false
}

func splitTemplateRef(ref string) (id, version string, ok bool) {
	id, version, found := strings.Cut(strings.TrimSpace(ref), "/")
	if !found || id == "" || version == "" || strings.Contains(id, "/") || strings.Contains(version, "/") {
		return "", "", false
	}
	return id, version, true
}

func findTemplateSpec(specs []appthingsv2.TemplateSpec, id, version string) (appthingsv2.TemplateSpec, bool) {
	for _, spec := range specs {
		if spec.Template.ID == id && spec.Template.Version == version {
			return spec, true
		}
	}
	return appthingsv2.TemplateSpec{}, false
}

func composeCreateModel(ctx context.Context, r *http.Request, app thingsV2App, tenant, template string, submitted map[string][]string, errorMessage string) (featuresthingsv2.ThingV2CreateViewModel, error) {
	model := featuresthingsv2.ThingV2CreateViewModel{ErrorMessage: errorMessage}
	model.Tenants = tokenTenants(r)

	if tenant == "" && len(model.Tenants) == 1 {
		tenant = model.Tenants[0]
	}
	model.Tenant = tenant

	templates, err := app.ThingsV2().ListTemplates(ctx, "", "")
	if err != nil {
		return featuresthingsv2.ThingV2CreateViewModel{}, err
	}
	for _, spec := range templates {
		label := spec.Template.DisplayName
		if label == "" {
			label = spec.Template.ID
		}
		model.Templates = append(model.Templates, featuresthings.TypeOption{
			Value: spec.Template.ID + "/" + spec.Template.Version,
			Label: fmt.Sprintf("%s %s", label, spec.Template.Version),
		})
	}

	templateID, templateVersion, templateOK := splitTemplateRef(template)
	spec, found := findTemplateSpec(templates, templateID, templateVersion)
	if !templateOK || !found {
		return model, nil
	}
	model.Template = template
	if spec.Template.DisplayName != "" {
		model.TemplateDisplay = fmt.Sprintf("%s %s", spec.Template.DisplayName, spec.Template.Version)
	} else {
		model.TemplateDisplay = template
	}

	variants, err := app.ThingsV2().ListVariants(ctx, tenant)
	if err != nil {
		return featuresthingsv2.ThingV2CreateViewModel{}, err
	}
	variantParams := map[string]float64{}
	selectedVariant := submittedValue(submitted, "variant")
	for _, variant := range variants {
		if variant.Variant.TemplateID != templateID || variant.Variant.TemplateVersion != templateVersion {
			continue
		}
		label := variant.Variant.ID
		if variant.Variant.Version != "" {
			label += " " + variant.Variant.Version
		}
		model.Variants = append(model.Variants, featuresthings.TypeOption{
			Value: variant.Variant.ID + "/" + variant.Variant.Version,
			Label: label,
		})
		if variant.Variant.ID+"/"+variant.Variant.Version == selectedVariant {
			model.Variant = selectedVariant
			variantParams = variant.Variant.ParamValues
		}
	}

	model.Params = paramFieldsForVariant(spec, variantParams, nil, submitted)

	model.Name = submittedValue(submitted, "name")
	model.Description = submittedValue(submitted, "description")
	model.Latitude = submittedValue(submitted, "latitude")
	model.Longitude = submittedValue(submitted, "longitude")
	model.GeometryKinds = effectiveGeometryKinds(spec.Template.AllowedGeometries)
	model.GeometryMode = resolveGeometryMode(submitted, nil, spec.Template.AllowedGeometries, spec.Template.AllowNoLocation)
	model.GeometryJSON = resolveGeometryJSON(submitted, nil)
	model.AllowNoLocation = spec.Template.AllowNoLocation

	return model, nil
}

func submittedValue(submitted map[string][]string, key string) string {
	if len(submitted[key]) == 0 {
		return ""
	}
	return submitted[key][0]
}

// submittedOrStored föredrar inskickat värde när nyckeln finns (även tomt),
// annars lagrat värde. Används för fält som får tömmas (t.ex. koordinater).
func submittedOrStored(submitted map[string][]string, key, stored string) string {
	if submitted == nil {
		return stored
	}
	if values, ok := submitted[key]; ok && len(values) > 0 {
		return values[0]
	}
	return stored
}

// effectiveGeometryKinds normaliserar mallens lista (tom = punkt + polygon).
func effectiveGeometryKinds(kinds []string) []string {
	if len(kinds) == 0 {
		return []string{"Point", "Polygon"}
	}
	return kinds
}

// geometryKindAllowed speglar serverns default: tom lista = punkt + polygon.
func geometryKindAllowed(kinds []string, kind string) bool {
	if len(kinds) == 0 {
		return strings.EqualFold(kind, "Point") || strings.EqualFold(kind, "Polygon")
	}
	for _, k := range kinds {
		if strings.EqualFold(k, kind) {
			return true
		}
	}
	return false
}

// resolveGeometryMode väljer geometriläge: inskickat giltigt val vinner,
// annars lagrad plats, annars ingen plats (om tillåtet) eller första tillåtna.
func resolveGeometryMode(submitted map[string][]string, stored *appthingsv2.Location, kinds []string, allowNone bool) string {
	switch strings.TrimSpace(submittedValue(submitted, "geometryMode")) {
	case "polygon":
		if geometryKindAllowed(kinds, "Polygon") {
			return "polygon"
		}
	case "none":
		if allowNone {
			return "none"
		}
	case "point":
		if geometryKindAllowed(kinds, "Point") {
			return "point"
		}
	}
	if stored != nil && geometryKindAllowed(kinds, stored.Type) {
		if strings.EqualFold(stored.Type, "Polygon") {
			return "polygon"
		}
		return "point"
	}
	if stored == nil && allowNone {
		return "none"
	}
	if geometryKindAllowed(kinds, "Point") {
		return "point"
	}
	if geometryKindAllowed(kinds, "Polygon") {
		return "polygon"
	}
	return "point"
}

// resolveGeometryJSON ekar inskickad polygon, annars lagrad polygon.
func resolveGeometryJSON(submitted map[string][]string, stored *appthingsv2.Location) string {
	if raw := strings.TrimSpace(submittedValue(submitted, "geometry")); raw != "" {
		return raw
	}
	if stored != nil && strings.EqualFold(stored.Type, "Polygon") && len(stored.Coordinates) > 0 {
		return string(stored.Coordinates)
	}
	return ""
}

// parseLocationInput bygger plats från formuläret: polygon (geometryMode
// polygon + geometry-JSON), ingen plats (geometryMode none) eller punkt
// (lat/long, tomt ger nil). Servern avgör mot mallens regler.
func parseLocationInput(r *http.Request) (*appthingsv2.Location, string) {
	switch strings.TrimSpace(r.Form.Get("geometryMode")) {
	case "polygon":
		return parsePolygonGeometry(r.Form.Get("geometry"))
	case "none":
		return nil, ""
	default:
		return parseOptionalLocation(r.Form.Get("latitude"), r.Form.Get("longitude"))
	}
}

// parsePolygonGeometry validerar ritad polygon ytligt (servern validerar strikt).
func parsePolygonGeometry(raw string) (*appthingsv2.Location, string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, "polygon needs at least 3 points"
	}
	var rings [][][]float64
	if err := json.Unmarshal([]byte(trimmed), &rings); err != nil || len(rings) == 0 {
		return nil, "invalid polygon geometry"
	}
	return &appthingsv2.Location{Type: "Polygon", Coordinates: json.RawMessage(trimmed)}, ""
}

// parseOptionalLocation bygger plats eller nil när båda fälten är tomma.
// Servern avgör mot mallens allowNoLocation; halvfyllt ger fältspecifikt fel.
func parseOptionalLocation(latitudeRaw, longitudeRaw string) (*appthingsv2.Location, string) {
	if strings.TrimSpace(latitudeRaw) == "" && strings.TrimSpace(longitudeRaw) == "" {
		return nil, ""
	}
	latitude, err := parseLatitude(latitudeRaw)
	if err != nil {
		return nil, err.Error()
	}
	longitude, err := parseLongitude(longitudeRaw)
	if err != nil {
		return nil, err.Error()
	}
	return &appthingsv2.Location{Type: "Point", Coordinates: pointCoordinates(longitude, latitude)}, ""
}

func buildCreateSpec(ctx context.Context, app thingsV2App, r *http.Request) (appthingsv2.ObjectSpec, string, string) {
	tenants := tokenTenants(r)
	tenant, ok := resolveCreateTenant(r, tenants, r.Form.Get("tenant"))
	if !ok {
		return appthingsv2.ObjectSpec{}, "", "tenant is required"
	}

	templateID, templateVersion, templateOK := splitTemplateRef(r.Form.Get("template"))
	if !templateOK {
		return appthingsv2.ObjectSpec{}, "", "template is required"
	}
	templates, err := app.ThingsV2().ListTemplates(ctx, "", "")
	if err != nil {
		return appthingsv2.ObjectSpec{}, "", "could not fetch templates"
	}
	specTemplate, found := findTemplateSpec(templates, templateID, templateVersion)
	if !found {
		return appthingsv2.ObjectSpec{}, "", "unknown template"
	}

	// Sak-ID:t genereras alltid serversidan; ett inskickat thingId ignoreras.
	thingID := uuid.NewString()
	name := strings.TrimSpace(r.Form.Get("name"))
	if name == "" {
		return appthingsv2.ObjectSpec{}, "", "name is required"
	}
	location, locFail := parseLocationInput(r)
	if locFail != "" {
		return appthingsv2.ObjectSpec{}, "", locFail
	}

	variantID, variantVersion := "", ""
	if raw := strings.TrimSpace(r.Form.Get("variant")); raw != "" {
		id, version, valid := splitTemplateRef(raw)
		if !valid {
			return appthingsv2.ObjectSpec{}, "", "unknown variant"
		}
		variants, err := app.ThingsV2().ListVariants(ctx, tenant)
		if err != nil {
			return appthingsv2.ObjectSpec{}, "", "could not fetch variants"
		}
		matched := false
		for _, variant := range variants {
			if variant.Variant.ID == id && variant.Variant.Version == version &&
				variant.Variant.TemplateID == templateID && variant.Variant.TemplateVersion == templateVersion {
				matched = true
				break
			}
		}
		if !matched {
			return appthingsv2.ObjectSpec{}, "", "variant does not belong to template"
		}
		variantID, variantVersion = id, version
	}

	overrides := map[string]float64{}
	for _, paramName := range specTemplate.Overridable {
		raw := strings.TrimSpace(r.Form.Get("param." + paramName))
		if raw == "" {
			continue
		}
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return appthingsv2.ObjectSpec{}, "", fmt.Sprintf("param %q is not a number", paramName)
		}
		if info, ok := specTemplate.ParamInfo[paramName]; ok {
			if info.Min != nil && value < *info.Min {
				return appthingsv2.ObjectSpec{}, "", fmt.Sprintf("param %q is below minimum %g", paramName, *info.Min)
			}
			if info.Max != nil && value > *info.Max {
				return appthingsv2.ObjectSpec{}, "", fmt.Sprintf("param %q is above maximum %g", paramName, *info.Max)
			}
		}
		overrides[paramName] = value
	}

	spec := appthingsv2.ObjectSpec{
		ThingID:         thingID,
		Name:            name,
		Location:        location,
		TemplateID:      templateID,
		TemplateVersion: templateVersion,
		VariantID:       variantID,
		VariantVersion:  variantVersion,
		Properties:      requiredProperties(specTemplate),
		ParamOverrides:  overrides,
	}
	if description := strings.TrimSpace(r.Form.Get("description")); description != "" {
		spec.Metadata = map[string]string{"description": description}
	}

	return spec, tenant, ""
}

func parseLatitude(raw string) (float64, error) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || value < -90 || value > 90 {
		return 0, fmt.Errorf("latitude must be a number between -90 and 90")
	}
	return value, nil
}

func parseLongitude(raw string) (float64, error) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || value < -180 || value > 180 {
		return 0, fmt.Errorf("longitude must be a number between -180 and 180")
	}
	return value, nil
}

func pointCoordinates(longitude, latitude float64) json.RawMessage {
	return json.RawMessage(fmt.Sprintf("[%g,%g]", longitude, latitude))
}

// requiredProperties tar med mallens obligatoriska egenskaper som tomma
// definitioner (servern prefillar presentationsfält från mallen).
func requiredProperties(spec appthingsv2.TemplateSpec) map[string]appthingsv2.Property {
	properties := make(map[string]appthingsv2.Property, len(spec.Template.Required))
	for _, id := range spec.Template.Required {
		properties[id] = appthingsv2.Property{}
	}
	return properties
}
