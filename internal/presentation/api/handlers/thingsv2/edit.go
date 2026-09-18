package thingsv2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/application/client"
	appthingsv2 "github.com/diwise/diwise-web/internal/application/thingsv2"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	featuresthings "github.com/diwise/diwise-web/internal/presentation/web/components/features/things"
	featuresthingsv2 "github.com/diwise/diwise-web/internal/presentation/web/components/features/thingsv2"
	v2layout "github.com/diwise/diwise-web/internal/presentation/web/components/layout"

	. "github.com/diwise/frontend-toolkit"
)

func NewThingsV2SavePage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app thingsV2App) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "things-v2",
		)

		if err := r.ParseForm(); err != nil {
			http.Error(w, "could not parse form data", http.StatusBadRequest)
			return
		}

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		renderEdit := func(model featuresthingsv2.ThingV2EditViewModel) {
			content := featuresthingsv2.ThingV2EditPage(localizer, model)
			page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
			if helpers.IsHxRequest(r) {
				page = v2layout.AppShell(localizer, assets, content)
			}
			helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, 0)
		}

		spec, tenant, revision, fail := buildEditSpec(ctx, app, r, id)
		if fail != "" {
			model, err := composeEditModel(ctx, r, app, id, r.Form, fail)
			if err != nil {
				http.Error(w, "could not fetch thing", http.StatusInternalServerError)
				return
			}
			renderEdit(model)
			return
		}

		if _, err := app.ThingsV2().UpdateThing(ctx, tenant, id, spec, revision); err != nil {
			message := err.Error()
			if errors.Is(err, client.ErrConflict) {
				message = localizer.Get("saveconflict")
			}
			model, modelErr := composeEditModel(ctx, r, app, id, r.Form, message)
			if modelErr != nil {
				http.Error(w, "could not fetch thing", http.StatusInternalServerError)
				return
			}
			renderEdit(model)
			return
		}

		target := fmt.Sprintf("/things-v2/%s?tenant=%s", id, tenant)
		if helpers.IsHxRequest(r) {
			w.Header().Set("HX-Redirect", target)
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, target, http.StatusFound)
	}

	return http.HandlerFunc(fn)
}

func composeEditModel(ctx context.Context, r *http.Request, app thingsV2App, id string, submitted map[string][]string, errorMessage string) (featuresthingsv2.ThingV2EditViewModel, error) {
	model := featuresthingsv2.ThingV2EditViewModel{ThingID: id, ErrorMessage: errorMessage}

	tenant := submittedValue(submitted, "tenant")
	if tenant == "" {
		var err error
		tenant, err = resolveDetailsTenant(r)
		if err != nil {
			return featuresthingsv2.ThingV2EditViewModel{}, err
		}
	}
	model.Tenant = tenant

	thing, err := app.ThingsV2().GetThing(ctx, tenant, id)
	if err != nil {
		return featuresthingsv2.ThingV2EditViewModel{}, err
	}
	config, err := app.ThingsV2().GetConfig(ctx, tenant, id)
	if err != nil {
		return featuresthingsv2.ThingV2EditViewModel{}, err
	}

	templates, err := app.ThingsV2().ListTemplates(ctx, "", "")
	if err != nil {
		return featuresthingsv2.ThingV2EditViewModel{}, err
	}
	spec, found := findTemplateSpec(templates, config.TemplateID, config.TemplateVersion)
	if !found {
		return featuresthingsv2.ThingV2EditViewModel{}, fmt.Errorf("unknown template %s %s", config.TemplateID, config.TemplateVersion)
	}
	if spec.Template.DisplayName != "" {
		model.TemplateDisplay = fmt.Sprintf("%s %s", spec.Template.DisplayName, spec.Template.Version)
	} else {
		model.TemplateDisplay = fmt.Sprintf("%s %s", config.TemplateID, config.TemplateVersion)
	}
	model.TemplateRef = config.TemplateID + "/" + config.TemplateVersion

	variants, err := app.ThingsV2().ListVariants(ctx, tenant)
	if err != nil {
		return featuresthingsv2.ThingV2EditViewModel{}, err
	}
	selectedVariant := submittedValue(submitted, "variant")
	if selectedVariant == "" && config.VariantID != "" {
		selectedVariant = config.VariantID + "/" + config.VariantVersion
	}
	variantParams := map[string]float64{}
	for _, variant := range variants {
		if variant.Variant.TemplateID != config.TemplateID || variant.Variant.TemplateVersion != config.TemplateVersion {
			continue
		}
		ref := variant.Variant.ID + "/" + variant.Variant.Version
		label := variant.Variant.ID
		if variant.Variant.Version != "" {
			label += " " + variant.Variant.Version
		}
		model.Variants = append(model.Variants, featuresthings.TypeOption{
			Value: ref,
			Label: label,
		})
		if ref == selectedVariant {
			model.Variant = ref
			variantParams = variant.Variant.ParamValues
		}
	}

	stored := map[string]float64{}
	for name, value := range config.ParamValues {
		if config.ParamSources[name] == "thing" {
			stored[name] = value
		}
	}
	model.Params = paramFieldsForVariant(spec, variantParams, stored, submitted)

	model.Name = firstNonEmpty(submittedValue(submitted, "name"), config.Name)
	description := config.Metadata["description"]
	if submitted != nil {
		if values, ok := submitted["description"]; ok && len(values) > 0 {
			description = values[0]
		}
	}
	model.Description = description
	model.Latitude = submittedOrStored(submitted, "latitude", formatEditCoordinate(config.Location, true))
	model.Longitude = submittedOrStored(submitted, "longitude", formatEditCoordinate(config.Location, false))

	if raw := submittedValue(submitted, "revision"); raw != "" {
		if revision, err := strconv.ParseInt(raw, 10, 64); err == nil {
			model.Revision = revision
		} else {
			model.Revision = thing.Revision
		}
	} else {
		model.Revision = thing.Revision
	}

	return model, nil
}

func buildEditSpec(ctx context.Context, app thingsV2App, r *http.Request, id string) (appthingsv2.ObjectSpec, string, int64, string) {
	tenants := tokenTenants(r)
	tenant, ok := resolveCreateTenant(r, tenants, r.Form.Get("tenant"))
	if !ok {
		return appthingsv2.ObjectSpec{}, "", 0, "tenant is required"
	}

	revision, err := strconv.ParseInt(strings.TrimSpace(r.Form.Get("revision")), 10, 64)
	if err != nil || revision < 1 {
		return appthingsv2.ObjectSpec{}, "", 0, "revision is required"
	}

	config, err := app.ThingsV2().GetConfig(ctx, tenant, id)
	if err != nil {
		return appthingsv2.ObjectSpec{}, "", 0, "could not fetch thing"
	}

	name := strings.TrimSpace(r.Form.Get("name"))
	if name == "" {
		return appthingsv2.ObjectSpec{}, "", 0, "name is required"
	}
	location, locFail := parseOptionalLocation(r.Form.Get("latitude"), r.Form.Get("longitude"))
	if locFail != "" {
		return appthingsv2.ObjectSpec{}, "", 0, locFail
	}

	templates, err := app.ThingsV2().ListTemplates(ctx, "", "")
	if err != nil {
		return appthingsv2.ObjectSpec{}, "", 0, "could not fetch templates"
	}
	specTemplate, found := findTemplateSpec(templates, config.TemplateID, config.TemplateVersion)
	if !found {
		return appthingsv2.ObjectSpec{}, "", 0, "unknown template"
	}

	variantID, variantVersion := "", ""
	if raw := strings.TrimSpace(r.Form.Get("variant")); raw != "" {
		variantID, variantVersion, ok = splitVariantRef(raw)
		if !ok {
			return appthingsv2.ObjectSpec{}, "", 0, "unknown variant"
		}
		variants, err := app.ThingsV2().ListVariants(ctx, tenant)
		if err != nil {
			return appthingsv2.ObjectSpec{}, "", 0, "could not fetch variants"
		}
		matched := false
		for _, variant := range variants {
			if variant.Variant.ID == variantID && variant.Variant.Version == variantVersion &&
				variant.Variant.TemplateID == config.TemplateID && variant.Variant.TemplateVersion == config.TemplateVersion {
				matched = true
				break
			}
		}
		if !matched {
			return appthingsv2.ObjectSpec{}, "", 0, "variant does not belong to template"
		}
	}

	overrides := map[string]float64{}
	for _, paramName := range specTemplate.Overridable {
		raw := strings.TrimSpace(r.Form.Get("param." + paramName))
		if raw == "" {
			continue
		}
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return appthingsv2.ObjectSpec{}, "", 0, fmt.Sprintf("param %q is not a number", paramName)
		}
		if info, ok := specTemplate.ParamInfo[paramName]; ok {
			if info.Min != nil && value < *info.Min {
				return appthingsv2.ObjectSpec{}, "", 0, fmt.Sprintf("param %q is below minimum %g", paramName, *info.Min)
			}
			if info.Max != nil && value > *info.Max {
				return appthingsv2.ObjectSpec{}, "", 0, fmt.Sprintf("param %q is above maximum %g", paramName, *info.Max)
			}
		}
		overrides[paramName] = value
	}

	metadata := map[string]string{}
	for key, value := range config.Metadata {
		metadata[key] = value
	}
	if description := strings.TrimSpace(r.Form.Get("description")); description != "" {
		metadata["description"] = description
	} else {
		delete(metadata, "description")
	}

	spec := appthingsv2.ObjectSpec{
		ThingID:         id,
		Name:            name,
		Category:        config.Category,
		Location:        location,
		Metadata:        metadata,
		TemplateID:      config.TemplateID,
		TemplateVersion: config.TemplateVersion,
		VariantID:       variantID,
		VariantVersion:  variantVersion,
		Properties:      config.Properties,
		Bindings:        config.Bindings,
		ParamOverrides:  overrides,
		Relations:       config.Relations,
	}

	return spec, tenant, revision, ""
}

func splitVariantRef(raw string) (id, version string, ok bool) {
	return splitTemplateRef(raw)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func formatEditCoordinate(location *appthingsv2.Location, latitude bool) string {
	if location == nil {
		return ""
	}
	longitude, latitudeValue, ok := location.Point()
	if !ok {
		return ""
	}
	if latitude {
		return strconv.FormatFloat(latitudeValue, 'f', -1, 64)
	}
	return strconv.FormatFloat(longitude, 'f', -1, 64)
}
