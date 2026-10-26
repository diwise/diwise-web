package rules

import (
	"context"
	"net/http"

	. "github.com/diwise/frontend-toolkit"
)

// NewRuleTogglePage växlar aktiv/inaktiv utan att radera: läser färsk regel,
// vänder Disabled och sparar med färsk revision (If-Match). Seed-regler med
// tom tenant nekas av backend (403 via writeServiceError); övriga fel blir
// 500 med anroparens text. Lyckad växling redirectar till listan.
func NewRuleTogglePage(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app rulesApp) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		m, err := app.Transforms().GetModel(r.Context(), "", id)
		if err != nil {
			writeServiceError(w, err, "could not fetch rule")
			return
		}

		rule := m.Rule
		rule.Disabled = !rule.Disabled

		if _, err := app.Transforms().UpdateModel(r.Context(), "", id, rule, m.Revision); err != nil {
			writeServiceError(w, err, "could not update rule")
			return
		}

		http.Redirect(w, r, "/rules?notice=updated", http.StatusFound)
	}

	return http.HandlerFunc(fn)
}
