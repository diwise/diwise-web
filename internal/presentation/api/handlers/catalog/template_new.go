package catalog

import (
	"context"
	"errors"
	"fmt"
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
		if err != nil {
			return nil, fmt.Errorf("bad parameter value %q", line)
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
			http.Error(w, "could not fetch template", http.StatusInternalServerError)
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
	if submitted != nil {
		model.Tenant = firstForm(submitted, "tenant")
		model.From = firstForm(submitted, "from")
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
	}

	return model, nil
}

func firstForm(form map[string][]string, key string) string {
	if len(form[key]) == 0 {
		return ""
	}
	return strings.TrimSpace(form[key][0])
}

func NewTemplateCreatePage(_ context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app catalogApp) http.HandlerFunc {
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

		renderErr := func(msg string) {
			model, err := composeTemplateNewModel(ctx, r, app, form)
			if err != nil {
				http.Error(w, "could not fetch template", http.StatusInternalServerError)
				return
			}
			// Bevara inmatningen över felet.
			model.Tenant = firstForm(form, "tenant")
			model.From = firstForm(form, "from")
			model.ID = firstForm(form, "id")
			model.Version = firstForm(form, "version")
			model.DisplayName = firstForm(form, "displayName")
			model.Description = firstForm(form, "description")
			model.Category = firstForm(form, "category")
			model.Required = firstForm(form, "required")
			model.Optional = firstForm(form, "optional")
			model.ParamDefaults = firstForm(form, "paramDefaults")
			model.OverridableText = firstForm(form, "overridable")
			model.Geometries = form["geometry"]
			model.AllowNoLocation = firstForm(form, "allowNoLocation") == "true"
			model.ErrorMessage = msg
			content := featurecatalog.TemplateNewPage(localizer, model)
			helpers.WriteComponentResponse(ctx, w, r, content, 32*1024, http.StatusOK)
		}

		tenant := firstForm(form, "tenant")
		if tenant == "" {
			renderErr("tenant is required")
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
			renderErr(err.Error())
			return
		}

		if err := app.Catalog().PublishTemplate(ctx, tenant, spec); err != nil {
			if errors.Is(err, client.ErrConflict) {
				renderErr("versionen finns redan, välj nytt versionsnummer")
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

// buildTemplateSpec bygger publiceringsspecen: kopia av From (ny version +
// redigerade fält; relations/recipes/propertyDefs/paramInfo följer med
// read-only) eller blankett utan källa.
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

	return spec, nil
}
