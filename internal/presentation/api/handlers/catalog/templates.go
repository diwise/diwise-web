package catalog

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/application/catalog"
	"github.com/diwise/diwise-web/internal/application/client"
	appthingsv2 "github.com/diwise/diwise-web/internal/application/thingsv2"
	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	featurecatalog "github.com/diwise/diwise-web/internal/presentation/web/components/features/catalog"
	featuresthings "github.com/diwise/diwise-web/internal/presentation/web/components/features/things"
	v2layout "github.com/diwise/diwise-web/internal/presentation/web/components/layout"

	. "github.com/diwise/frontend-toolkit"
)

type catalogApp interface {
	Catalog() *catalog.Service
	ThingsV2() *appthingsv2.Service
	GetTenants(ctx context.Context) []string
}

// tokenTenants returnerar tokenens unika tenants (routen är redan skyddad
// med rätt scope).
func tokenTenants(r *http.Request) []string {
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

// resolveCatalogTenant väljer tenant: explicit ?tenant= vinner, annars den
// enda token-auktoriserade tenanten. Flera utan val ger "" (väljare visas).
func resolveCatalogTenant(r *http.Request) string {
	if tenant := strings.TrimSpace(r.URL.Query().Get("tenant")); tenant != "" {
		return tenant
	}
	if tenants := tokenTenants(r); len(tenants) == 1 {
		return tenants[0]
	}
	return ""
}

func NewTemplatesPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app catalogApp) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "catalog",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		model, err := composeTemplatesModel(ctx, r, app)
		if err != nil {
			http.Error(w, "could not fetch templates", http.StatusInternalServerError)
			return
		}

		content := featurecatalog.TemplatesPage(localizer, model)
		page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
		if helpers.IsHxRequest(r) {
			page = v2layout.AppShell(localizer, assets, content)
		}

		helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func composeTemplatesModel(ctx context.Context, r *http.Request, app catalogApp) (featurecatalog.TemplatesPageViewModel, error) {
	model := featurecatalog.TemplatesPageViewModel{
		Tenants:   tokenTenants(r),
		Tenant:    resolveCatalogTenant(r),
		Category:  strings.TrimSpace(r.URL.Query().Get("category")),
		Templates: []featurecatalog.TemplateRowViewModel{},
	}

	if model.Tenant == "" {
		return model, nil
	}

	specs, err := app.ThingsV2().ListTemplates(ctx, model.Tenant, model.Category)
	if err != nil {
		return featurecatalog.TemplatesPageViewModel{}, err
	}

	seen := map[string]bool{}
	for _, spec := range specs {
		label := spec.Template.DisplayName
		if label == "" {
			label = spec.Template.ID
		}
		model.Templates = append(model.Templates, featurecatalog.TemplateRowViewModel{
			ID:          spec.Template.ID,
			Version:     spec.Template.Version,
			Category:    spec.Template.Category,
			DisplayName: label,
			Description: spec.Template.Description,
		})
		if spec.Template.Category != "" && !seen[spec.Template.Category] {
			seen[spec.Template.Category] = true
			model.CategoryOptions = append(model.CategoryOptions, featuresthings.TypeOption{
				Value: spec.Template.Category,
				Label: spec.Template.Category,
			})
		}
	}

	return model, nil
}

func NewTemplateDetailsPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app catalogApp) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		ver := r.PathValue("version")
		if id == "" || ver == "" || strings.Contains(id, "/") || strings.Contains(ver, "/") {
			http.Error(w, "invalid template reference", http.StatusBadRequest)
			return
		}

		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "catalog",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))

		tenant := resolveCatalogTenant(r)
		if tenant == "" {
			http.Error(w, "tenant is required", http.StatusBadRequest)
			return
		}

		spec, err := app.Catalog().GetTemplate(ctx, tenant, id, ver)
		if err != nil {
			if errors.Is(err, client.ErrNotFound) {
				http.Error(w, "template not found", http.StatusNotFound)
				return
			}
			http.Error(w, "could not fetch template", http.StatusInternalServerError)
			return
		}

		content := featurecatalog.TemplateDetailsPage(localizer, featurecatalog.TemplateDetailsViewModel{
			Tenant: tenant,
			Spec:   spec,
		})
		page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
		if helpers.IsHxRequest(r) {
			page = v2layout.AppShell(localizer, assets, content)
		}

		helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, 0)
	}

	return http.HandlerFunc(fn)
}
