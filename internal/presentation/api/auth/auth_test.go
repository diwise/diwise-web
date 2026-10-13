package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

const scopedPolicy = `package example.authz
default allow := false
allow := {"access": {
  "sensors-tenant": ["sensors.read", "sensors.create"],
  "things-tenant": ["things.read"],
  "rules-tenant": ["transforms.read"]
}} if { input.token == "good" }
`

func TestEndpointFilteringPreservesNavigationScopes(t *testing.T) {
	a, err := NewAuthenticator(t.Context(), strings.NewReader(scopedPolicy))
	if err != nil {
		t.Fatal(err)
	}
	for _, optional := range []bool{false, true} {
		t.Run(map[bool]string{false: "required", true: "optional"}[optional], func(t *testing.T) {
			mw := a.RequireAccess("sensors.read")
			if optional {
				mw = a.OptionalAuth(AnyScope)
			}
			h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				if !LoggedIn(ctx) || Token(ctx) != "good" {
					t.Error("authenticated request lost its token or access")
				}
				for _, scope := range []Scope{"sensors.read", "sensors.create", "things.read", "transforms.read"} {
					if !HasScope(ctx, scope) {
						t.Errorf("navigation lost %s", scope)
					}
				}
				if !HasScopeInTenant(ctx, "things-tenant", "things.read") || HasScopeInTenant(ctx, "sensors-tenant", "things.read") {
					t.Error("tenant-specific scopes were mixed")
				}
				if !optional {
					if got := GetTenantsWithAllowedScopes(ctx, AnyScope); !slices.Equal(got, []string{"sensors-tenant"}) {
						t.Errorf("endpoint tenants broadened: %v", got)
					}
					if len(GetTenantsWithAllowedScopes(ctx, "things.read")) != 0 {
						t.Error("navigation rights broadened the endpoint's tenant set")
					}
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			r := httptest.NewRequest(http.MethodGet, "/sensors", nil)
			r.Header.Set("Authorization", "Bearer good")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusNoContent {
				t.Fatalf("status %d", w.Code)
			}
		})
	}
}

func TestAccessObjectDefaultAndExplicitLegacy(t *testing.T) {
	policy := `package example.authz
allow := {"tenants": ["legacy-tenant"]}`
	for _, legacy := range []bool{false, true} {
		var opts []Option
		if legacy {
			opts = append(opts, WithAccessObjectAuthorization(false))
		}
		a, err := NewAuthenticator(t.Context(), strings.NewReader(policy), opts...)
		if err != nil {
			t.Fatal(err)
		}
		h := a.RequireAccess("sensors.read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if HasScope(r.Context(), "sensors.read") {
				t.Error("legacy tenant list fabricated a navigation scope")
			}
			if got := GetTenantsWithAllowedScopes(r.Context(), "sensors.read"); !slices.Equal(got, []string{"legacy-tenant"}) {
				t.Errorf("legacy endpoint access changed: %v", got)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		r := httptest.NewRequest(http.MethodGet, "/sensors", nil)
		r.Header.Set("Authorization", "Bearer good")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		want := http.StatusInternalServerError
		if legacy {
			want = http.StatusNoContent
		}
		if w.Code != want {
			t.Errorf("legacy=%t: status %d, want %d", legacy, w.Code, want)
		}
	}
}

func TestScopesAreExactAndDeniedRequestsStayDenied(t *testing.T) {
	ctx := WithAccess(context.Background(), accessMap{"tenant": scopesToSet([]Scope{"sensors.*", "sensors.update"})})
	if HasScope(ctx, "sensors.read") || HasScope(context.Background(), "sensors.read") {
		t.Fatal("missing exact scope must not grant navigation")
	}
	a, err := NewAuthenticator(t.Context(), strings.NewReader(scopedPolicy))
	if err != nil {
		t.Fatal(err)
	}
	for _, scopes := range [][]Scope{{"sensors.update"}, {"sensors.read", "things.read"}} {
		h := a.RequireAccess(scopes...)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Error("unauthorized handler called")
		}))
		r := httptest.NewRequest(http.MethodPost, "/sensors", nil)
		r.Header.Set("Authorization", "Bearer good")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Errorf("scopes %v: status %d", scopes, w.Code)
		}
	}
}
