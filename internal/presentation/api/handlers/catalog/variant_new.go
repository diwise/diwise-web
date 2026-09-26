package catalog

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/application/client"
	appthingsv2 "github.com/diwise/diwise-web/internal/application/thingsv2"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	featurecatalog "github.com/diwise/diwise-web/internal/presentation/web/components/features/catalog"
	v2layout "github.com/diwise/diwise-web/internal/presentation/web/components/layout"

	. "github.com/diwise/frontend-toolkit"
)

func NewVariantNewPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app catalogApp) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "catalog",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		model, err := composeVariantNewModel(ctx, r, app, nil)
		if err != nil {
			http.Error(w, "could not fetch variant", http.StatusInternalServerError)
			return
		}

		content := featurecatalog.VariantNewPage(localizer, model)
		page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
		if helpers.IsHxRequest(r) {
			page = v2layout.AppShell(localizer, assets, content)
		}

		helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func composeVariantNewModel(ctx context.Context, r *http.Request, app catalogApp, submitted map[string][]string) (featurecatalog.VariantNewViewModel, error) {
	model := featurecatalog.VariantNewViewModel{
		Tenants: tokenTenants(r),
		Tenant:  resolveCatalogTenant(r),
		From:    strings.TrimSpace(r.URL.Query().Get("from")),
	}
	if submitted != nil {
		model.Tenant = firstForm(submitted, "tenant")
		model.From = firstForm(submitted, "from")
	}

	tenant := model.Tenant
	if tenant == "" && len(model.Tenants) == 1 {
		tenant = model.Tenants[0]
	}
	if tenant != "" {
		if submitted == nil {
			model.Tenant = tenant
		}
		// Mallväljare: alla publicerade versioner som datalist.
		if specs, err := app.ThingsV2().ListTemplates(ctx, tenant, ""); err == nil {
			for _, spec := range specs {
				model.TemplateOptions = append(model.TemplateOptions,
					spec.Template.ID+"/"+spec.Template.Version)
			}
		}
		if model.From != "" && submitted == nil {
			fromID, fromVersion, ok := splitRef(model.From)
			if !ok {
				return model, nil
			}
			if spec, err := app.Catalog().GetVariant(ctx, tenant, fromID, fromVersion); err == nil {
				model.ID = spec.Variant.ID
				model.TemplateRef = spec.Variant.TemplateID + "/" + spec.Variant.TemplateVersion
				model.ParamValues = formatKeyValues(spec.Variant.ParamValues)
			}
		}
	}

	return model, nil
}

func NewVariantCreatePage(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app catalogApp) http.HandlerFunc {
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
			model, err := composeVariantNewModel(ctx, r, app, form)
			if err != nil {
				http.Error(w, "could not fetch variant", http.StatusInternalServerError)
				return
			}
			model.Tenant = firstForm(form, "tenant")
			model.From = firstForm(form, "from")
			model.ID = firstForm(form, "id")
			model.Version = firstForm(form, "version")
			model.TemplateRef = firstForm(form, "templateRef")
			model.ParamValues = firstForm(form, "paramValues")
			model.ErrorMessage = msg
			helpers.WriteComponentResponse(ctx, w, r, featurecatalog.VariantNewPage(localizer, model), 32*1024, http.StatusOK)
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
		templateID, templateVersion, ok := splitRef(firstForm(form, "templateRef"))
		if !ok {
			renderErr("template reference must be id/version")
			return
		}
		params, err := parseKeyValues(firstForm(form, "paramValues"))
		if err != nil {
			renderErr(err.Error())
			return
		}

		spec := appthingsv2.VariantSpec{Variant: appthingsv2.Variant{
			ID: id, Version: version,
			TemplateID: templateID, TemplateVersion: templateVersion,
			ParamValues: params,
		}}
		if err := app.Catalog().PublishVariant(ctx, tenant, spec); err != nil {
			if errors.Is(err, client.ErrConflict) {
				renderErr("versionen finns redan, välj nytt versionsnummer")
				return
			}
			renderErr(err.Error())
			return
		}

		target := "/catalog/variants/" + id + "/" + version + "?tenant=" + tenant
		http.Redirect(w, r, target, http.StatusFound)
	}

	return http.HandlerFunc(fn)
}
