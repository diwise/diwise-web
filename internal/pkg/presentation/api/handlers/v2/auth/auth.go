package auth

import (
	"net/http"
	"net/url"
)

const postLogoutRedirectCookie = "diwise-v2-post-logout"

func NewLoginRedirect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target := r.URL.Query().Get("path")
		if target == "" {
			target = "/v2/home"
		}

		http.Redirect(w, r, "/login?path="+url.QueryEscape(target), http.StatusFound)
	}
}

func NewLogoutRedirect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     postLogoutRedirectCookie,
			Value:    "/v2/home",
			Path:     "/",
			HttpOnly: true,
			MaxAge:   120,
		})

		http.Redirect(w, r, "/logout", http.StatusFound)
	}
}

func RedirectIfPostLogout(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(postLogoutRedirectCookie)
		if err == nil && cookie.Value != "" && r.URL.Path == "/" {
			http.SetCookie(w, &http.Cookie{
				Name:     postLogoutRedirectCookie,
				Value:    "",
				Path:     "/",
				HttpOnly: true,
				MaxAge:   -1,
			})
			http.Redirect(w, r, cookie.Value, http.StatusFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}
