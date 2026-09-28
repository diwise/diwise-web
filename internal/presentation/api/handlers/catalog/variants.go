package catalog

import (
	"context"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	featurecatalog "github.com/diwise/diwise-web/internal/presentation/web/components/features/catalog"
	v2layout "github.com/diwise/diwise-web/internal/presentation/web/components/layout"

	. "github.com/diwise/frontend-toolkit"
)

func NewVariantsPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app catalogApp) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "catalog",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))

		tenant := resolveCatalogTenant(r)
		model := featurecatalog.VariantsPageViewModel{
			Tenants:           tokenTenants(r),
			Tenant:            tenant,
			Variants:          []featurecatalog.VariantRowViewModel{},
			CatalogConfigured: app.Catalog().Configured(),
		}
		if tenant != "" && model.CatalogConfigured {
			specs, err := app.ThingsV2().ListVariants(ctx, tenant)
			if err != nil {
				writeServiceError(w, err, "could not fetch variants")
				return
			}
			for _, spec := range specs {
				model.Variants = append(model.Variants, featurecatalog.VariantRowViewModel{
					ID:              spec.Variant.ID,
					Version:         spec.Variant.Version,
					TemplateID:      spec.Variant.TemplateID,
					TemplateVersion: spec.Variant.TemplateVersion,
				})
			}
		}

		content := featurecatalog.VariantsPage(localizer, model)
		page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
		if helpers.IsHxRequest(r) {
			page = v2layout.AppShell(localizer, assets, content)
		}

		helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewVariantDetailsPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app catalogApp) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		ver := r.PathValue("version")
		if id == "" || ver == "" || strings.Contains(id, "/") || strings.Contains(ver, "/") {
			http.Error(w, "invalid variant reference", http.StatusBadRequest)
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

		spec, err := app.Catalog().GetVariant(ctx, tenant, id, ver)
		if err != nil {
			writeServiceError(w, err, "could not fetch variant")
			return
		}

		content := featurecatalog.VariantDetailsPage(localizer, featurecatalog.VariantDetailsViewModel{
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
