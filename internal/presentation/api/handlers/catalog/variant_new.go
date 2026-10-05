package catalog

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
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
			writeServiceError(w, err, "could not fetch variant")
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
	if submitted == nil && model.Tenant != "" && len(model.Tenants) > 0 && !slices.Contains(model.Tenants, model.Tenant) {
		return model, client.ErrForbidden
	}
	if submitted != nil {
		model.Tenant = firstForm(submitted, "tenant")
		model.From = firstForm(submitted, "from")
		return model, nil
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
		} else {
			return model, err
		}
		if model.From != "" && submitted == nil {
			fromID, fromVersion, ok := splitRef(model.From)
			if !ok {
				return model, fmt.Errorf("invalid from reference")
			}
			if spec, err := app.Catalog().GetVariant(ctx, tenant, fromID, fromVersion); err == nil {
				model.ID = spec.Variant.ID
				model.TemplateRef = spec.Variant.TemplateID + "/" + spec.Variant.TemplateVersion
				model.ParamValues = formatKeyValues(spec.Variant.ParamValues)
				model.PresetProperties = strings.Join(spec.PresetProperties, "\n")
			} else {
				return model, err
			}
		}
	}

	return model, nil
}

func NewVariantCreatePage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app catalogApp) http.HandlerFunc {
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
			model, err := composeVariantNewModel(ctx, r, app, form)
			if err != nil {
				writeServiceError(w, err, "could not fetch variant")
				return
			}
			model.Tenant = firstForm(form, "tenant")
			model.From = firstForm(form, "from")
			model.ID = submittedRaw(form, "id")
			model.Version = submittedRaw(form, "version")
			model.TemplateRef = submittedRaw(form, "templateRef")
			model.ParamValues = submittedRaw(form, "paramValues")
			model.PresetProperties = submittedRaw(form, "presetProperties")
			model.ErrorMessage = msg
			content := featurecatalog.VariantNewPage(localizer, model)
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
		if from := firstForm(form, "from"); from != "" {
			fromID, fromVersion, valid := splitRef(from)
			if !valid || fromID != id {
				renderErr("variant id is locked to the copied version")
				return
			}
			copied, err := app.Catalog().GetVariant(ctx, tenant, fromID, fromVersion)
			if err != nil {
				if errors.Is(err, client.ErrUnauthorized) || errors.Is(err, client.ErrForbidden) {
					renderErr(err.Error(), errorStatus(err))
				} else {
					renderErr(err.Error())
				}
				return
			}
			spec.PresetProperties = copied.PresetProperties
		}
		if _, exists := form["presetProperties"]; exists {
			spec.PresetProperties = parseLines(firstForm(form, "presetProperties"))
		}
		if err := app.Catalog().PublishVariant(ctx, tenant, spec); err != nil {
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

		target := "/catalog/variants/" + id + "/" + version + "?tenant=" + tenant
		http.Redirect(w, r, target, http.StatusFound)
	}

	return http.HandlerFunc(fn)
}
