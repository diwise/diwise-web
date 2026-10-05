package rules

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/diwise/diwise-web/internal/application/client"
	apptransform "github.com/diwise/diwise-web/internal/application/transform"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	featurerules "github.com/diwise/diwise-web/internal/presentation/web/components/features/rules"

	. "github.com/diwise/frontend-toolkit"
)

// parsePreviewForm bygger regeln + preview-eventet från formuläret:
// previewKind/previewMessage/previewSensorType vid sidan av regelfälten.
// Meddelandet måste vara exakt ett JSON-dokument (samma krav som backend).
func parsePreviewForm(form map[string][]string) (apptransform.Rule, apptransform.PreviewEvent, error) {
	rule, err := parseRuleForm(form)
	if err != nil {
		return rule, apptransform.PreviewEvent{}, err
	}

	first := func(key string) string {
		if len(form[key]) == 0 {
			return ""
		}
		return strings.TrimSpace(form[key][0])
	}

	kind := first("previewKind")
	raw := first("previewMessage")
	if kind == "" {
		return rule, apptransform.PreviewEvent{}, errors.New("preview kind is required")
	}
	if raw == "" {
		return rule, apptransform.PreviewEvent{}, errors.New("preview message is required")
	}
	event, err := apptransform.NewPreviewEvent(kind, raw, first("previewSensorType"))
	return rule, event, err
}

// NewValidateFragment torrvaliderar formulärregeln utan att spara (HTMX,
// RequireHX). Spara kräver alltid servervalidering — ingen klientsides
// Validate-duplikation som kan drifta (5.3).
func NewValidateFragment(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app rulesApp) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		if err := r.ParseForm(); err != nil {
			helpers.WriteComponentResponse(r.Context(), w, r, featurerules.ValidateResult(localizer, featurerules.ValidateResultViewModel{Message: "invalid form"}), 16*1024, http.StatusOK)
			return
		}
		rule, err := parseRuleForm(map[string][]string(r.Form))
		if err != nil {
			if r.Form.Get("editorAction") != "" {
				w.Header().Set("HX-Retarget", "#validate-result")
				w.Header().Set("HX-Reswap", "outerHTML")
			}
			helpers.WriteComponentResponse(r.Context(), w, r, featurerules.ValidateResult(localizer, featurerules.ValidateResultViewModel{Message: err.Error()}), 16*1024, http.StatusOK)
			return
		}
		if r.Form.Get("editorAction") == "export" {
			helpers.WriteComponentResponse(r.Context(), w, r, featurerules.RuleJSONField(featurerules.RuleFormViewModel{Rule: rule}), 32*1024, http.StatusOK)
			return
		}
		if r.Form.Get("editorAction") == "import" {
			id := r.Form.Get("ruleID")
			revision, err := parseRevision(r.Form.Get("revision"))
			if id != "" && err != nil {
				http.Error(w, "revision is required", http.StatusBadRequest)
				return
			}
			model := featurerules.RuleFormViewModel{ID: id, Revision: revision, IsNew: id == "", ShowDelete: id != "", Rule: rule,
				Tenants: ruleTokenTenants(r), TransformConfigured: app.Transforms().Configured(), SubmittedForm: map[string][]string{
					"previewKind": {r.Form.Get("previewKind")}, "previewMessage": {r.Form.Get("previewMessage")}, "previewSensorType": {r.Form.Get("previewSensorType")},
				}}
			fillTypeOptions(r.Context(), app, rule.Match.Tenant, &model)
			fillMatchOptions(r.Context(), app, &model)
			helpers.WriteComponentResponse(r.Context(), w, r, featurerules.RuleFormPage(localizer, model), 32*1024, http.StatusOK)
			return
		}
		if r.Form.Get("checkCompatibility") == "true" {
			if !slices.Contains(ruleTokenTenants(r), rule.Match.Tenant) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			result, err := app.Transforms().CheckCompatibility(r.Context(), rule.Match.Tenant, apptransform.CompatibilityRequest{
				TemplateID: rule.Match.Type, TemplateVersion: rule.Match.ThingTypeVersion,
				VariantID: rule.Match.VariantID, VariantVersion: rule.Match.VariantVersion, Rules: []apptransform.Rule{rule},
			})
			model := featurerules.CompatibilityViewModel{Result: result, CandidateName: featurerules.RuleRowViewModel{Name: rule.Name, ID: r.Form.Get("ruleID")}.DisplayName()}
			if err != nil {
				if errors.Is(err, client.ErrUnauthorized) || errors.Is(err, client.ErrForbidden) {
					writeServiceError(w, err, "")
					return
				}
				model.ErrorMessage = err.Error()
				if errors.Is(err, client.ErrNotFound) {
					model.ErrorMessage = "Compatibility API is not available; rule validation is not a compatibility guarantee."
				}
			}
			helpers.WriteComponentResponse(r.Context(), w, r, featurerules.CompatibilityResult(model), 32*1024, http.StatusOK)
			return
		}

		if err := app.Transforms().ValidateRule(r.Context(), "", rule); err != nil {
			// 401/403 ger status för AccessDenied-toast; övrigt visas i badgen.
			if errors.Is(err, client.ErrUnauthorized) || errors.Is(err, client.ErrForbidden) {
				writeServiceError(w, err, "")
				return
			}
			msg := err.Error()
			if verr, ok := errors.AsType[*apptransform.ValidationError](err); ok {
				msg = verr.Message
			}
			helpers.WriteComponentResponse(r.Context(), w, r, featurerules.ValidateResult(localizer, featurerules.ValidateResultViewModel{Message: msg}), 16*1024, http.StatusOK)
			return
		}

		helpers.WriteComponentResponse(r.Context(), w, r, featurerules.ValidateResult(localizer, featurerules.ValidateResultViewModel{OK: true}), 16*1024, http.StatusOK)
	}

	return http.HandlerFunc(fn)
}

// NewPreviewFragment torrkör formulärregeln mot exempelhändelsen (HTMX,
// RequireHX). Matchad men undertryckt regel (required saknas) förklaras i
// klartext via skippedReason.
func NewPreviewFragment(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app rulesApp) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		if err := r.ParseForm(); err != nil {
			helpers.WriteComponentResponse(r.Context(), w, r, featurerules.PreviewResult(localizer, featurerules.PreviewResultViewModel{ErrorMessage: "invalid form"}), 16*1024, http.StatusOK)
			return
		}
		rule, event, err := parsePreviewForm(map[string][]string(r.Form))
		if err != nil {
			helpers.WriteComponentResponse(r.Context(), w, r, featurerules.PreviewResult(localizer, featurerules.PreviewResultViewModel{ErrorMessage: err.Error()}), 16*1024, http.StatusOK)
			return
		}

		res, err := app.Transforms().PreviewRule(r.Context(), "", rule, event)
		if err != nil {
			if errors.Is(err, client.ErrUnauthorized) || errors.Is(err, client.ErrForbidden) {
				writeServiceError(w, err, "")
				return
			}
			msg := err.Error()
			if verr, ok := errors.AsType[*apptransform.ValidationError](err); ok {
				msg = verr.Message
			}
			helpers.WriteComponentResponse(r.Context(), w, r, featurerules.PreviewResult(localizer, featurerules.PreviewResultViewModel{ErrorMessage: msg}), 16*1024, http.StatusOK)
			return
		}

		helpers.WriteComponentResponse(r.Context(), w, r, featurerules.PreviewResult(localizer, featurerules.PreviewResultViewModel{Result: res, Event: event, Queried: true, CandidateName: featurerules.RuleRowViewModel{Name: rule.Name, ID: r.Form.Get("ruleID")}.DisplayName()}), 32*1024, http.StatusOK)
	}

	return http.HandlerFunc(fn)
}

// NewFixtureFragment returnerar ett exempel-message som text (HTMX,
// RequireHX): fyller preview-textarean vid fixture-val. Läser
// `previewFixture` (formulärfältets namn, det htmx skickar) med `name` som
// fallback för direktlänkar.
func NewFixtureFragment(_ context.Context, _ LocaleBundle, _ AssetLoaderFunc, _ rulesApp) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		name := strings.TrimSpace(q.Get("previewFixture"))
		if name == "" {
			name = strings.TrimSpace(q.Get("name"))
		}
		f, ok := featurerules.FixtureByName(name)
		if !ok {
			http.Error(w, "unknown fixture", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(f.Message))
	}

	return http.HandlerFunc(fn)
}
