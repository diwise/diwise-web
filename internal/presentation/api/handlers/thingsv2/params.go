package thingsv2

import (
	"context"
	"net/http"
	"strconv"

	appthingsv2 "github.com/diwise/diwise-web/internal/application/thingsv2"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	featuresthingsv2 "github.com/diwise/diwise-web/internal/presentation/web/components/features/thingsv2"

	. "github.com/diwise/frontend-toolkit"
)

// NewThingsV2ParamsComponent laddar om parameterfälten när varianten
// byts: fälten fylls med valda variantens värden (mallens default annars).
// Inmatade värden nollställs vid variantbyte per design.
func NewThingsV2ParamsComponent(_ context.Context, _ LocaleBundle, _ AssetLoaderFunc, app thingsV2App) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(r.Context())

		templateID, templateVersion, ok := splitTemplateRef(r.URL.Query().Get("template"))
		if !ok {
			http.Error(w, "template is required", http.StatusBadRequest)
			return
		}

		templates, err := app.ThingsV2().ListTemplates(ctx, "", "")
		if err != nil {
			http.Error(w, "could not fetch templates", http.StatusInternalServerError)
			return
		}
		spec, found := findTemplateSpec(templates, templateID, templateVersion)
		if !found {
			http.Error(w, "unknown template", http.StatusBadRequest)
			return
		}

		variants, err := app.ThingsV2().ListVariants(ctx, r.URL.Query().Get("tenant"))
		if err != nil {
			http.Error(w, "could not fetch variants", http.StatusInternalServerError)
			return
		}
		variantParams := map[string]float64{}
		for _, variant := range variants {
			if variant.Variant.TemplateID == templateID && variant.Variant.TemplateVersion == templateVersion &&
				variant.Variant.ID+"/"+variant.Variant.Version == r.URL.Query().Get("variant") {
				variantParams = variant.Variant.ParamValues
				break
			}
		}

		prefix := r.URL.Query().Get("prefix")
		if prefix == "" {
			prefix = "new"
		}
		component := featuresthingsv2.ThingV2ParamFieldsBlock(
			paramFieldsForVariant(spec, variantParams, nil, nil), prefix)
		helpers.WriteComponentResponse(ctx, w, r, component, 8*1024, 0)
	}

	return http.HandlerFunc(fn)
}

// paramFieldsForVariant bygger parameterfält för en mall + vald variant.
// Värdeprioritet: inskickat > lagrat thing-värde > visad default
// (variantens värde, annars mallens).
func paramFieldsForVariant(spec appthingsv2.TemplateSpec, variantParams, stored map[string]float64, submitted map[string][]string) []featuresthingsv2.ParamFieldViewModel {
	fields := make([]featuresthingsv2.ParamFieldViewModel, 0, len(spec.Overridable))
	for _, name := range spec.Overridable {
		field := featuresthingsv2.ParamFieldViewModel{Name: name, Value: submittedValue(submitted, "param."+name)}
		if info, ok := spec.ParamInfo[name]; ok {
			field.Label = info.Description
			field.Unit = info.Unit
			field.Min = info.Min
			field.Max = info.Max
		}
		if field.Value == "" {
			if value, ok := stored[name]; ok {
				field.Value = strconv.FormatFloat(value, 'f', -1, 64)
			}
		}
		if value, ok := variantParams[name]; ok {
			field.Default = value
			field.HasDefault = true
		} else if value, ok := spec.ParamDefaults[name]; ok {
			field.Default = value
			field.HasDefault = true
		}
		fields = append(fields, field)
	}
	return fields
}
