package authz

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"slices"

	"github.com/open-policy-agent/opa/rego"
)

type loggedInKey string
type tokenKey string
type tenantsKey string
type rolesKey string

const AuthToken tokenKey = "jwt-token"
const LoggedIn loggedInKey = "logged-in"
const AllowedTenants tenantsKey = "allowed-tenants"
const Roles rolesKey = "roles"

func NewContextFromAuthorizationHeader(ctx context.Context, r *http.Request) (context.Context, error) {
	authHeader, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if authHeader != "" {
		ctx = context.WithValue(ctx, LoggedIn, "yes")
		ctx = context.WithValue(ctx, AuthToken, authHeader)
	}
	return ctx, nil
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, err := NewContextFromAuthorizationHeader(r.Context(), r)
		if err == nil {
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

func NewAuthenticator(ctx context.Context, logger *slog.Logger, policies io.Reader) (func(http.Handler) http.Handler, error) {
	module, err := io.ReadAll(policies)
	if err != nil {
		return nil, fmt.Errorf("unable to read authz policies: %s", err.Error())
	}

	query, err := rego.New(
		rego.Query("x = data.example.authz.allow"),
		rego.Module("example.rego", string(module)),
	).PrepareForEval(ctx)

	if err != nil {
		return nil, err
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var err error

			token := r.Header.Get("Authorization")

			if token == "" || !strings.HasPrefix(token, "Bearer ") {
				next.ServeHTTP(w, r)
				return
			}

			path := strings.Split(r.URL.Path, "/")

			input := map[string]any{
				"method": r.Method,
				"path":   path[1:],
				"token":  token[7:],
			}

			results, err := query.Eval(r.Context(), rego.EvalInput(input))
			if err != nil {
				logger.Error("opa eval failed", "err", err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if len(results) == 0 {
				err = errors.New("opa query could not be satisfied")
				logger.Error("auth failed", "err", err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			} else {

				binding := results[0].Bindings["x"]

				// If authz fails we will get back a single bool. Check for that first.
				allowed, ok := binding.(bool)
				if ok && !allowed {
					err = errors.New("authorization failed")
					logger.Warn(err.Error())
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}

				// If authz succeeds we should expect a result object here
				result, ok := binding.(map[string]any)

				if !ok {
					err = errors.New("unexpected result type")
					logger.Error("opa error", "err", err.Error())
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				toStr := func(m map[string]any, key string) ([]string, bool) {
					value, ok := m[key]
					if !ok {
						return nil, false
					}
					strs, ok2 := value.([]any)
					if !ok2 {
						return nil, false
					}
					result := make([]string, len(strs))
					for idx, s := range strs {
						result[idx] = s.(string)
					}

					return result, true
				}

				tenants, ok := toStr(result, "tenants")
				if !ok {
					err = errors.New("bad response from authz policy engine")
					logger.Error("opa error", "key", "tenants", "err", err.Error())
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				roles, ok := toStr(result, "roles")
				if !ok {
					err = errors.New("bad response from authz policy engine")
					logger.Error("opa error", "key", "roles", "err", err.Error())
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				ctx := context.WithValue(context.WithValue(r.Context(), AllowedTenants, tenants), Roles, roles)
				r = r.WithContext(ctx)
			}

			// Token is authenticated, pass it through
			next.ServeHTTP(w, r)
		})
	}, nil

}

func IsLoggedIn(ctx context.Context) bool {
	if value, ok := ctx.Value(LoggedIn).(string); ok {
		return value == "yes"
	}
	return false
}

func Token(ctx context.Context) string {
	if token, ok := ctx.Value(AuthToken).(string); ok {
		return token
	}

	return ""
}

const (
	RoleCreateSensor RoleName = "create_sensor"
	RoleUpdateSensor RoleName = "update_sensor"
	RoleDeleteSensor RoleName = "delete_sensor"
	RoleCreateThing  RoleName = "create_thing"
	RoleUpdateThing  RoleName = "update_thing"
	RoleDeleteThing  RoleName = "delete_thing"
	RoleAdmin        RoleName = "admin"
)

type RoleName string

func IsInRole(ctx context.Context, role RoleName) bool {
	if roles, ok := ctx.Value(Roles).([]string); ok {
		roleName := string(role)
		if slices.Contains(roles, roleName) {
			return true
		}
	}
	return false
}
