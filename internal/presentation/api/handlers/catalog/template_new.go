package catalog

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/application/client"
	appthingsv2 "github.com/diwise/diwise-web/internal/application/thingsv2"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	featurecatalog "github.com/diwise/diwise-web/internal/presentation/web/components/features/catalog"
	v2layout "github.com/diwise/diwise-web/internal/presentation/web/components/layout"

	. "github.com/diwise/frontend-toolkit"
)

// validRef kontrollerar id/version-format (icke-tomt, inget "/") före
// backend-anrop — samma kontrakt som serverns PutTemplate/PutVariant.
func validRef(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && !strings.Contains(s, "/")
}

func splitRef(ref string) (id, version string, ok bool) {
	id, version, found := strings.Cut(strings.TrimSpace(ref), "/")
	if !found || !validRef(id) || !validRef(version) {
		return "", "", false
	}
	return id, version, true
}

func parseLines(s string) []string {
	out := []string{}
	for line := range strings.Lines(s) {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

func parseKeyValues(s string) (map[string]float64, error) {
	out := map[string]float64{}
	for _, line := range parseLines(s) {
		k, v, found := strings.Cut(line, "=")
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if !found || k == "" {
			return nil, fmt.Errorf("bad parameter line %q (want key=value)", line)
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return nil, fmt.Errorf("bad parameter value %q", line)
		}
		if _, exists := out[k]; exists {
			return nil, fmt.Errorf("duplicate parameter %q", k)
		}
		out[k] = f
	}
	return out, nil
}

func formatKeyValues(m map[string]float64) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, k+"="+strconv.FormatFloat(m[k], 'f', -1, 64))
	}
	return strings.Join(lines, "\n")
}

func NewTemplateNewPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app catalogApp) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "catalog",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		model, err := composeTemplateNewModel(ctx, r, app, nil)
		if err != nil {
			writeServiceError(w, err, "could not fetch template")
			return
		}

		content := featurecatalog.TemplateNewPage(localizer, model)
		page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
		if helpers.IsHxRequest(r) {
			page = v2layout.AppShell(localizer, assets, content)
		}

		helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func composeTemplateNewModel(ctx context.Context, r *http.Request, app catalogApp, submitted map[string][]string) (featurecatalog.TemplateNewViewModel, error) {
	model := featurecatalog.TemplateNewViewModel{
		Tenants: tokenTenants(r),
		Tenant:  resolveCatalogTenant(r),
		From:    strings.TrimSpace(r.URL.Query().Get("from")),
	}
	model.RelationsStructured = true
	model.RelationRows = []featurecatalog.RelationEditRow{{}}
	if submitted == nil && model.Tenant != "" && len(model.Tenants) > 0 && !slices.Contains(model.Tenants, model.Tenant) {
		return model, client.ErrForbidden
	}
	if submitted != nil {
		model.Tenant = firstForm(submitted, "tenant")
		model.From = firstForm(submitted, "from")
		model.FromLocked = model.From != ""
		return model, nil // Error rendering must not depend on another backend read.
	}

	if model.From != "" {
		fromID, fromVersion, ok := splitRef(model.From)
		if !ok {
			return model, fmt.Errorf("invalid from reference %q", model.From)
		}
		tenant := model.Tenant
		if tenant == "" && len(model.Tenants) == 1 {
			tenant = model.Tenants[0]
		}
		if tenant == "" {
			return model, nil
		}
		if submitted == nil {
			model.Tenant = tenant
		}
		spec, err := app.Catalog().GetTemplate(ctx, tenant, fromID, fromVersion)
		if err != nil {
			return model, err
		}
		model.FromLocked = true
		model.ID = spec.Template.ID
		model.DisplayName = spec.Template.DisplayName
		model.Description = spec.Template.Description
		model.Category = spec.Template.Category
		model.Required = strings.Join(spec.Template.Required, "\n")
		model.Optional = strings.Join(spec.Template.Optional, "\n")
		model.Geometries = append([]string(nil), spec.Template.AllowedGeometries...)
		model.AllowNoLocation = spec.Template.AllowNoLocation
		model.ParamDefaults = formatKeyValues(spec.ParamDefaults)
		model.OverridableText = strings.Join(spec.Overridable, "\n")
		model.CopiedRelations = len(spec.Template.Relations)
		model.CopiedRecipes = len(spec.Recipes)
		model.RelationRows = nil
		templateFields(&model, spec)
	}

	return model, nil
}

func firstForm(form map[string][]string, key string) string {
	if len(form[key]) == 0 {
		return ""
	}
	return strings.TrimSpace(form[key][0])
}

func NewTemplateCreatePage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app catalogApp) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "catalog",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))

		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		form := map[string][]string(r.Form)

		renderErr := func(msg string, statuses ...int) {
			model, err := composeTemplateNewModel(ctx, r, app, form)
			if err != nil {
				writeServiceError(w, err, "could not fetch template")
				return
			}
			// Bevara inmatningen över felet.
			model.Tenant = firstForm(form, "tenant")
			model.From = firstForm(form, "from")
			model.ID = submittedRaw(form, "id")
			model.Version = submittedRaw(form, "version")
			model.DisplayName = submittedRaw(form, "displayName")
			model.Description = submittedRaw(form, "description")
			model.Category = submittedRaw(form, "category")
			model.Required = submittedRaw(form, "required")
			model.Optional = submittedRaw(form, "optional")
			model.ParamDefaults = submittedRaw(form, "paramDefaults")
			model.OverridableText = submittedRaw(form, "overridable")
			model.Geometries = form["geometry"]
			model.AllowNoLocation = firstForm(form, "allowNoLocation") == "true"
			submittedTemplateFields(&model, form)
			model.ErrorMessage = msg
			content := featurecatalog.TemplateNewPage(localizer, model)
			page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
			if helpers.IsHxRequest(r) {
				page = v2layout.AppShell(localizer, assets, content)
			}
			status := http.StatusOK
			if len(statuses) > 0 {
				status = statuses[0]
			}
			helpers.WriteComponentResponse(ctx, formStatusWriter{w, status}, r, page, 32*1024, 0)
		}

		tenant := firstForm(form, "tenant")
		if tenants := tokenTenants(r); len(tenants) > 0 && !slices.Contains(tenants, tenant) {
			renderErr("tenant is not authorized", http.StatusForbidden)
			return
		}
		if tenant == "" {
			renderErr("tenant is required")
			return
		}
		if firstForm(form, "addRelation") == "true" {
			renderErr("")
			return
		}
		id := firstForm(form, "id")
		version := firstForm(form, "version")
		if !validRef(id) || !validRef(version) {
			renderErr("id and version are required (no slashes)")
			return
		}

		spec, err := buildTemplateSpec(ctx, app, tenant, form, id, version)
		if err != nil {
			if errors.Is(err, client.ErrUnauthorized) || errors.Is(err, client.ErrForbidden) {
				renderErr(err.Error(), errorStatus(err))
				return
			}
			renderErr(err.Error())
			return
		}

		if err := app.Catalog().PublishTemplate(ctx, tenant, spec); err != nil {
			if errors.Is(err, client.ErrConflict) {
				renderErr("versionen finns redan, välj nytt versionsnummer: " + err.Error())
				return
			}
			if errors.Is(err, client.ErrUnauthorized) || errors.Is(err, client.ErrForbidden) {
				renderErr(err.Error(), errorStatus(err))
				return
			}
			renderErr(err.Error())
			return
		}

		target := "/catalog/templates/" + id + "/" + version + "?tenant=" + tenant
		http.Redirect(w, r, target, http.StatusFound)
	}

	return http.HandlerFunc(fn)
}

// buildTemplateSpec copies the pinned source before applying submitted fields.
// Omitted advanced fields preserve the source; explicitly empty fields clear it.
func buildTemplateSpec(ctx context.Context, app catalogApp, tenant string, form map[string][]string, id, version string) (appthingsv2.TemplateSpec, error) {
	var spec appthingsv2.TemplateSpec
	if from := firstForm(form, "from"); from != "" {
		fromID, fromVersion, ok := splitRef(from)
		if !ok {
			return spec, fmt.Errorf("invalid from reference %q", from)
		}
		var err error
		spec, err = app.Catalog().GetTemplate(ctx, tenant, fromID, fromVersion)
		if err != nil {
			return spec, err
		}
		if spec.Template.ID != id {
			return spec, fmt.Errorf("template id is locked to the copied version")
		}
	} else {
		spec = appthingsv2.TemplateSpec{}
		spec.Template.ID = id
	}

	spec.Template.Version = version
	spec.Template.DisplayName = firstForm(form, "displayName")
	spec.Template.Description = firstForm(form, "description")
	spec.Template.Category = firstForm(form, "category")
	spec.Template.Required = parseLines(firstForm(form, "required"))
	spec.Template.Optional = parseLines(firstForm(form, "optional"))
	spec.Template.AllowedGeometries = form["geometry"]
	spec.Template.AllowNoLocation = firstForm(form, "allowNoLocation") == "true"

	params, err := parseKeyValues(firstForm(form, "paramDefaults"))
	if err != nil {
		return spec, err
	}
	spec.ParamDefaults = params
	spec.Overridable = parseLines(firstForm(form, "overridable"))
	if _, ok := form["primaryProperty"]; ok {
		spec.Template.PrimaryProperty = firstForm(form, "primaryProperty")
	}
	if _, ok := form["repeatGroups"]; ok {
		spec.Template.RepeatGroups = parseLines(firstForm(form, "repeatGroups"))
	}
	if _, ok := form["allowedQuantities"]; ok {
		spec.Template.AllowedQuantities = parseLines(firstForm(form, "allowedQuantities"))
	}
	if _, ok := form["labels"]; ok {
		spec.Template.Labels = parseLines(firstForm(form, "labels"))
	}
	for _, field := range []struct {
		name string
		dest any
	}{
		{"propertyDefs", &spec.Template.PropertyDefs},
		{"relations", &spec.Template.Relations},
		{"recipes", &spec.Recipes},
		{"paramInfo", &spec.ParamInfo},
		{"operatorVersions", &spec.OperatorVersions},
	} {
		if field.name == "relations" && firstForm(form, "relationRowsPresent") == "true" {
			continue
		}
		if err := decodeField(form, field.name, field.dest); err != nil {
			return spec, err
		}
	}
	if firstForm(form, "relationRowsPresent") == "true" {
		relations, err := parseRelationRows(form)
		if err != nil {
			return spec, err
		}
		spec.Template.Relations = relations
	}

	return spec, nil
}
