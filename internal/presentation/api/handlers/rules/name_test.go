package rules

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	featurerules "github.com/diwise/diwise-web/internal/presentation/web/components/features/rules"
)

func TestRuleNameOptionalAndPreservedOnParseError(t *testing.T) {
	for _, name := range []string{"", "  ", "  Name with punctuation <&> / arbitrary text  "} {
		form := previewForm(url.Values{"name": {name}})
		rule, err := parseRuleForm(form)
		if err != nil || rule.Name != strings.TrimSpace(name) {
			t.Fatalf("name %q: %+v %v", name, rule, err)
		}
		form.Set("priority", "invalid")
		rule, err = parseRuleForm(form)
		if err == nil {
			t.Fatal("expected parse error")
		}
		back := renderedForm(t, featurerules.RuleFormViewModel{IsNew: true, TransformConfigured: true, Rule: rule, SubmittedForm: form})
		if back.Get("name") != name {
			t.Fatalf("submitted name lost: %q", back.Get("name"))
		}
	}
}

func TestRuleNameConflictAndCreateErrorHTML(t *testing.T) {
	app, done := testApp(t)
	defer done()
	for _, save := range []bool{false, true} {
		form := previewForm(url.Values{"name": {"User's rule <&>"}, "revision": {"0"}})
		if !save {
			form.Set("priority", "invalid")
		}
		req := authorizedRequest(form)
		req.Header.Set("HX-Request", "true")
		req.SetPathValue("id", "22222222-2222-2222-2222-222222222222")
		rec := httptest.NewRecorder()
		handler := NewRuleCreatePage(context.Background(), testLocaleBundle(), testAssets(), app)
		if save {
			handler = NewRuleSavePage(context.Background(), testLocaleBundle(), testAssets(), app)
		}
		handler(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `name="name" value="User&#39;s rule &lt;&amp;&gt;"`) {
			t.Fatalf("error/conflict lost name: %d %s", rec.Code, rec.Body.String())
		}
		if save && !strings.Contains(rec.Body.String(), "rules_conflict") {
			t.Fatal("expected revision conflict")
		}
	}
}
