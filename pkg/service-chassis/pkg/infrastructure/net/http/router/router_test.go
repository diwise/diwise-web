package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/diwise/service-chassis/pkg/infrastructure/net/http/router"
	"github.com/matryer/is"
)

func TestRegisterRoutes(t *testing.T) {
	is := is.New(t)

	mux := http.NewServeMux()
	r := router.New(mux)

	r.Route("/api", func(r router.ServeMux) {
		r.Route("/test", func(r router.ServeMux) {
			r.Get("/bananas", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/test/bananas")
	is.NoErr(err)
	is.Equal(resp.StatusCode, http.StatusOK)
}

func TestRegisterRouteWithPathValue(t *testing.T) {
	is := is.New(t)

	mux := http.NewServeMux()
	r := router.New(mux)

	bananaID := ""

	r.Route("/api", func(r router.ServeMux) {
		r.Route("/test", func(r router.ServeMux) {
			r.Route("/bananas", func(r router.ServeMux) {
				r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
					bananaID = r.PathValue("id")
					w.WriteHeader(http.StatusOK)
				})
			})
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/test/bananas/42")
	is.NoErr(err)
	is.Equal(resp.StatusCode, http.StatusOK)
	is.Equal(bananaID, "42")
}

func TestRegisterGroupedRoutes(t *testing.T) {
	is := is.New(t)

	mux := http.NewServeMux()
	r := router.New(mux)

	r.Route("/api", func(r router.ServeMux) {
		r.Route("/v1", func(r router.ServeMux) {
			r.Group(func(r router.ServeMux) {
				r.Get("", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNoContent)
				})
				r.Route("/test", func(r router.ServeMux) {
					r.Get("/oranges", func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)
					})
				})
			})
		})
		r.Route("/v2", func(r router.ServeMux) {
			r.Group(func(r router.ServeMux) {
				r.Route("/test", func(r router.ServeMux) {
					r.Get("/oranges", func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusPaymentRequired)
					})
				})
			})
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1")
	is.NoErr(err)
	is.Equal(resp.StatusCode, http.StatusNoContent) // should be able to reach without trailing slash

	resp, err = http.Get(ts.URL + "/api/v1/")
	is.NoErr(err)
	is.Equal(resp.StatusCode, http.StatusNoContent) // should be able to reach with trailing slash

	resp, err = http.Get(ts.URL + "/api/v2/test/oranges")
	is.NoErr(err)
	is.Equal(resp.StatusCode, http.StatusPaymentRequired)
}

func TestMultipleMethodsOnSameRoute(t *testing.T) {
	is := is.New(t)

	mux := http.NewServeMux()
	r := router.New(mux)

	r.Route("/api", func(r router.ServeMux) {
		r.Route("/test", func(r router.ServeMux) {
			r.Get("", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusTeapot)
			})
			r.Head("", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})
			r.Post("", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusCreated)
			})
			r.Post("/", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusPaymentRequired)
			})
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/api/test")
	is.Equal(resp.StatusCode, http.StatusTeapot)

	resp, _ = http.Head(ts.URL + "/api/test")
	is.Equal(resp.StatusCode, http.StatusNoContent)

	resp, _ = http.Post(ts.URL+"/api/test", "application/json", nil)
	is.Equal(resp.StatusCode, http.StatusCreated)

	resp, _ = http.Post(ts.URL+"/api/test/", "application/json", nil)
	is.Equal(resp.StatusCode, http.StatusPaymentRequired) // POST with trailing slash should hit different endpoint
}

func TestAddsSlashesAutomatically(t *testing.T) {
	is := is.New(t)

	mux := http.NewServeMux()
	r := router.New(mux)

	r.Route("a", func(r router.ServeMux) {
		r.Route("b", func(r router.ServeMux) {
			r.Get("c", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/a/b/c")
	is.Equal(resp.StatusCode, http.StatusOK)
}

func TestWithAndWithoutSlash(t *testing.T) {
	is := is.New(t)

	mux := http.NewServeMux()
	r := router.New(mux)

	r.Route("admin", func(r router.ServeMux) {
		r.Get("", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusFound)
		})

		r.Route("/", func(r router.ServeMux) {
			r.Get("", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/admin")
	is.Equal(resp.StatusCode, http.StatusFound)

	resp, _ = http.Get(ts.URL + "/admin/")
	is.Equal(resp.StatusCode, http.StatusNoContent)
}

func TestCallsUsedMiddleware(t *testing.T) {
	is := is.New(t)

	mux := http.NewServeMux()
	r := router.New(mux)

	mwcalls := 0

	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mwcalls++
			next.ServeHTTP(w, r)
		})
	}

	r.Route("api", func(r router.ServeMux) {
		r.Group(func(r router.ServeMux) {
			r.Use(middleware)
			r.Group(func(r router.ServeMux) {
				r.Get("", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				})
			})
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/api")
	is.Equal(resp.StatusCode, http.StatusOK)
	is.Equal(mwcalls, 1)
}

func TestCallsUsedMiddlewareForEmptyAndSlashGroupRoots(t *testing.T) {
	is := is.New(t)

	mux := http.NewServeMux()
	r := router.New(mux)

	mwcalls := 0

	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mwcalls++
			next.ServeHTTP(w, r)
		})
	}

	r.Route("api", func(r router.ServeMux) {
		r.Use(middleware)
		r.Group(func(r router.ServeMux) {
			r.Get("", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusAccepted)
			})
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/api")
	is.Equal(resp.StatusCode, http.StatusAccepted)

	resp, _ = http.Get(ts.URL + "/api/")
	is.Equal(resp.StatusCode, http.StatusNoContent)

	is.Equal(mwcalls, 2)
}
