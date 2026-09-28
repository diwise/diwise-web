package rules

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/application/client"
	apptransform "github.com/diwise/diwise-web/internal/application/transform"
	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	featurerules "github.com/diwise/diwise-web/internal/presentation/web/components/features/rules"
	v2layout "github.com/diwise/diwise-web/internal/presentation/web/components/layout"

	. "github.com/diwise/frontend-toolkit"
)

type rulesApp interface {
	Transforms() *apptransform.Service
}

// ruleTokenTenants returnerar tokenens unika tenants.
func ruleTokenTenants(r *http.Request) []string {
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

// ruleSummary sammanfattar en regel för listan: sakregler visar
// händelse + typ ( + relation/lifecycle), mätvärdesregler objekt/miljö.
func ruleSummary(rule apptransform.Rule) string {
	m := rule.Match
	if m.Kind == "thing" {
		parts := []string{}
		if m.Event != "" {
			parts = append(parts, m.Event)
		}
		if m.Type != "" {
			parts = append(parts, m.Type)
		}
		if m.SubType != "" {
			parts = append(parts, m.SubType)
		}
		if m.Relation != "" {
			rel := m.Relation
			if m.RelationRemoved {
				rel += " (removed)"
			}
			parts = append(parts, rel)
		}
		if m.Lifecycle != "" {
			parts = append(parts, m.Lifecycle)
		}
		return strings.Join(parts, " ")
	}
	parts := []string{}
	if m.Object != "" {
		parts = append(parts, m.Object)
	}
	if m.Env != "" {
		parts = append(parts, m.Env)
	}
	if m.Device != "" {
		parts = append(parts, m.Device)
	}
	return strings.Join(parts, " ")
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// writeServiceError mappar klassificerbara backend-fel till status så att
// AccessDenied-middleware kan toasta/redirecta (401/403); 404 för okänt,
// 500 med anroparens text för övrigt.
func writeServiceError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, client.ErrUnauthorized):
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	case errors.Is(err, client.ErrForbidden):
		http.Error(w, "forbidden", http.StatusForbidden)
	case errors.Is(err, client.ErrNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	default:
		http.Error(w, fallback, http.StatusInternalServerError)
	}
}

func NewRulesPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app rulesApp) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "rules",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		model := featurerules.RulesPageViewModel{
			Tenants:             ruleTokenTenants(r),
			Kind:                strings.TrimSpace(r.URL.Query().Get("kind")),
			Event:               strings.TrimSpace(r.URL.Query().Get("event")),
			Tenant:              strings.TrimSpace(r.URL.Query().Get("tenant")),
			Source:              strings.TrimSpace(r.URL.Query().Get("source")),
			Search:              strings.TrimSpace(r.URL.Query().Get("search")),
			Rules:               []featurerules.RuleRowViewModel{},
			TransformConfigured: app.Transforms().Configured(),
		}
		notice := strings.TrimSpace(r.URL.Query().Get("notice"))
		if notice != "" {
			model.Notice = localizer.Get("rules_notice_" + notice)
		}

		if model.TransformConfigured {
			models, err := app.Transforms().ListModels(ctx, "", nil)
			if err != nil {
				writeServiceError(w, err, "could not fetch rules")
				return
			}
			for _, m := range models {
				if model.Kind != "" && m.Rule.Match.Kind != model.Kind {
					continue
				}
				if model.Event != "" && m.Rule.Match.Event != model.Event {
					continue
				}
				if model.Tenant != "" && m.Rule.Match.Tenant != model.Tenant {
					continue
				}
				if model.Source != "" && m.Source != model.Source {
					continue
				}
				if model.Search != "" && !strings.Contains(strings.ToLower(m.Rule.Match.Type+m.Rule.Match.Event+m.ID), strings.ToLower(model.Search)) {
					continue
				}
				model.Rules = append(model.Rules, featurerules.RuleRowViewModel{
					ID:      m.ID,
					ShortID: shortID(m.ID),
					Kind:    m.Rule.Match.Kind,
					Summary: ruleSummary(m.Rule),
					Tenant:  m.Rule.Match.Tenant,
					Source:  m.Source,
				})
			}
		}

		content := featurerules.RulesPage(localizer, model)
		page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
		if helpers.IsHxRequest(r) {
			page = v2layout.AppShell(localizer, assets, content)
		}

		helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewRuleDetailsPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app rulesApp) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "rules",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))

		if !app.Transforms().Configured() {
			content := featurerules.RuleFormPage(localizer, featurerules.RuleFormViewModel{
				Tenants: ruleTokenTenants(r),
			})
			page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
			if helpers.IsHxRequest(r) {
				page = v2layout.AppShell(localizer, assets, content)
			}
			helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, 0)
			return
		}

		m, err := app.Transforms().GetModel(ctx, "", id)
		if err != nil {
			writeServiceError(w, err, "could not fetch rule")
			return
		}

		model := featurerules.RuleFormViewModel{
			ID:                  id,
			Revision:            m.Revision,
			Tenants:             ruleTokenTenants(r),
			Rule:                m.Rule,
			IsSeed:              m.Source == "seed",
			ShowDelete:          true,
			TransformConfigured: true,
		}
		if notice := strings.TrimSpace(r.URL.Query().Get("notice")); notice != "" {
			model.Notice = localizer.Get("rules_notice_" + notice)
		}

		content := featurerules.RuleFormPage(localizer, model)
		page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
		if helpers.IsHxRequest(r) {
			page = v2layout.AppShell(localizer, assets, content)
		}

		helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewRuleDeletePage(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app rulesApp) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "rules",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))

		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		revision, err := parseRevision(r.Form.Get("revision"))
		if err != nil {
			http.Error(w, "revision is required", http.StatusBadRequest)
			return
		}

		// Första POST visar bekräftelsen (B4 seed-varning); andra raderar.
		if r.Form.Get("confirm") != "yes" {
			m, err := app.Transforms().GetModel(ctx, "", id)
			if err != nil {
				writeServiceError(w, err, "could not fetch rule")
				return
			}
			content := featurerules.RuleFormPage(localizer, featurerules.RuleFormViewModel{
				ID:                  id,
				Revision:            revision,
				Tenants:             ruleTokenTenants(r),
				Rule:                m.Rule,
				IsSeed:              m.Source == "seed",
				ShowDelete:          true,
				ConfirmDelete:       true,
				TransformConfigured: app.Transforms().Configured(),
			})
			helpers.WriteComponentResponse(ctx, w, r, content, 32*1024, http.StatusOK)
			return
		}

		if err := app.Transforms().DeleteModel(ctx, "", id, revision); err != nil {
			// Redan borta i annat fönster = OK (idempotent budskap, 4.5).
			if errors.Is(err, client.ErrNotFound) {
				http.Redirect(w, r, "/rules?notice=deleted", http.StatusFound)
				return
			}
			if errors.Is(err, client.ErrConflict) {
				http.Error(w, "rule changed by another user", http.StatusConflict)
				return
			}
			writeServiceError(w, err, "could not delete rule")
			return
		}

		http.Redirect(w, r, "/rules?notice=deleted", http.StatusFound)
	}

	return http.HandlerFunc(fn)
}

func parseRevision(s string) (int64, error) {
	rev, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || rev < 0 {
		return 0, errInvalidRevision
	}
	return rev, nil
}

var errInvalidRevision = errors.New("invalid revision")
