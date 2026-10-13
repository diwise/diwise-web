package layout

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	frontend "github.com/diwise/frontend-toolkit"
	ftkmock "github.com/diwise/frontend-toolkit/mock"
)

func TestDesktopAndMobileNavigationUsesExactScopes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		access  map[string][]string
		visible []string
	}{
		{"none", map[string][]string{"t": {}}, nil},
		{"sensors", map[string][]string{"t": {"sensors.read"}}, []string{"/sensors"}},
		{"things", map[string][]string{"t": {"things.read"}}, []string{"/things-v2", "/catalog/templates"}},
		{"rules", map[string][]string{"t": {"transforms.read"}}, []string{"/rules"}},
		{"write-only", map[string][]string{"t": {"sensors.create", "things.update", "transforms.write"}}, nil},
		{"wildcard", map[string][]string{"t": {"sensors.*"}}, nil},
		{"different-tenants", map[string][]string{"a": {"sensors.read"}, "b": {"things.read"}, "c": {"transforms.read"}}, []string{"/sensors", "/things-v2", "/catalog/templates", "/rules"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			access, err := json.Marshal(tc.access)
			if err != nil {
				t.Fatal(err)
			}
			policy := fmt.Sprintf("package example.authz\nallow := {\"access\": %s}", access)
			a, err := auth.NewAuthenticator(t.Context(), strings.NewReader(policy))
			if err != nil {
				t.Fatal(err)
			}
			l10n := &ftkmock.LocalizerMock{GetFunc: func(key string) string { return key }}
			var body bytes.Buffer
			h := a.OptionalAuth(auth.AnyScope)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				if err := AppShell(l10n, frontend.AssetLoaderFunc(nil), templ.NopComponent).Render(r.Context(), &body); err != nil {
					t.Error(err)
				}
			}))
			r := httptest.NewRequest(http.MethodGet, "/home", nil)
			r.Header.Set("Authorization", "Bearer good")
			h.ServeHTTP(httptest.NewRecorder(), r)
			for _, href := range []string{"/sensors", "/things-v2", "/catalog/templates", "/rules"} {
				want := 0
				for _, visible := range tc.visible {
					if visible == href {
						want = 2
					}
				}
				if got := strings.Count(body.String(), `href="`+href+`"`); got != want {
					t.Errorf("%s: %d links, want %d (desktop and mobile)", href, got, want)
				}
			}
			for _, href := range []string{"/home", "/logout"} {
				if got := strings.Count(body.String(), `href="`+href+`"`); got != 2 {
					t.Errorf("%s: %d links, want 2", href, got)
				}
			}
		})
	}
}
