package rules

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

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
	var msg any
	dec := json.NewDecoder(strings.NewReader(raw))
	if err := dec.Decode(&msg); err != nil {
		return rule, apptransform.PreviewEvent{}, errors.New("preview message must be exactly one JSON document")
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return rule, apptransform.PreviewEvent{}, errors.New("preview message must be exactly one JSON document")
	}

	return rule, apptransform.PreviewEvent{
		Kind:       kind,
		Message:    msg,
		SensorType: first("previewSensorType"),
	}, nil
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
			helpers.WriteComponentResponse(r.Context(), w, r, featurerules.ValidateResult(localizer, featurerules.ValidateResultViewModel{Message: err.Error()}), 16*1024, http.StatusOK)
			return
		}

		if err := app.Transforms().ValidateRule(r.Context(), "", rule); err != nil {
			msg := err.Error()
			var verr *apptransform.ValidationError
			if errors.As(err, &verr) {
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
			msg := err.Error()
			var verr *apptransform.ValidationError
			if errors.As(err, &verr) {
				msg = verr.Message
			}
			helpers.WriteComponentResponse(r.Context(), w, r, featurerules.PreviewResult(localizer, featurerules.PreviewResultViewModel{ErrorMessage: msg}), 16*1024, http.StatusOK)
			return
		}

		helpers.WriteComponentResponse(r.Context(), w, r, featurerules.PreviewResult(localizer, featurerules.PreviewResultViewModel{Result: res, Queried: true}), 16*1024, http.StatusOK)
	}

	return http.HandlerFunc(fn)
}

// NewFixtureFragment returnerar ett exempel-message som text (HTMX,
// RequireHX): fyller preview-textarean vid fixture-val.
func NewFixtureFragment(_ context.Context, _ LocaleBundle, _ AssetLoaderFunc, _ rulesApp) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		f, ok := featurerules.FixtureByName(strings.TrimSpace(r.URL.Query().Get("name")))
		if !ok {
			http.Error(w, "unknown fixture", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(f.Message))
	}

	return http.HandlerFunc(fn)
}
