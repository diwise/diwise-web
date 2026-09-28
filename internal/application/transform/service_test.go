package transform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/matryer/is"
)

func testRule() Rule {
	return Rule{
		Match: Match{Kind: "thing", Event: "things.v1.values", Type: "room", Tenant: "t"},
		Entities: []Entity{{
			ID: "urn:ngsi-ld:Room:{{nameOrID}}", Type: "Room",
			Properties: []Property{{Target: "name", Type: "Text", Source: Source{Field: "name"}}},
		}},
	}
}

// stubTransform svarar som regel-API:t: CRUD under /models med revisioner,
// validate/preview som torra verktyg. Minns sista If-Match och query för
// assertions.
func stubTransform(t *testing.T, seen *map[string]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	model := Model{ID: "11111111-1111-1111-1111-111111111111", Revision: 1, Source: "api", Kind: "thing", Rule: testRule()}

	write := func(w http.ResponseWriter, code int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(v)
	}

	mux.HandleFunc("/models", func(w http.ResponseWriter, r *http.Request) {
		(*seen)["query"] = r.URL.RawQuery
		switch r.Method {
		case http.MethodGet:
			models := []Model{model}
			if kind := r.URL.Query().Get("kind"); kind != "" {
				models = nil
				if kind == "thing" {
					models = []Model{model}
				}
			}
			if models == nil {
				models = []Model{}
			}
			write(w, http.StatusOK, map[string]any{"models": models})
		case http.MethodPost:
			var rule Rule
			if err := json.NewDecoder(r.Body).Decode(&rule); err != nil || len(rule.Entities) == 0 {
				write(w, http.StatusBadRequest, map[string]string{"error": "rule must declare at least one entity"})
				return
			}
			model.Rule = rule
			write(w, http.StatusCreated, model)
		}
	})
	mux.HandleFunc("/models/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != model.ID {
			write(w, http.StatusNotFound, map[string]string{"error": "rule not found"})
			return
		}
		(*seen)["ifmatch"] = r.Header.Get("If-Match")
		switch r.Method {
		case http.MethodGet:
			write(w, http.StatusOK, model)
		case http.MethodPut:
			if r.Header.Get("If-Match") != `"rev-1"` {
				write(w, http.StatusConflict, map[string]string{"error": "revision conflict"})
				return
			}
			var rule Rule
			if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
				write(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
				return
			}
			model.Rule = rule
			model.Revision = 2
			write(w, http.StatusOK, model)
		case http.MethodDelete:
			if r.Header.Get("If-Match") != `"rev-1"` {
				write(w, http.StatusConflict, map[string]string{"error": "revision conflict"})
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}
	})
	mux.HandleFunc("/models/validate", func(w http.ResponseWriter, r *http.Request) {
		var rule Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil || len(rule.Entities) == 0 {
			write(w, http.StatusBadRequest, map[string]string{"error": "rule must declare at least one entity"})
			return
		}
		write(w, http.StatusOK, map[string]bool{"valid": true})
	})
	mux.HandleFunc("/models/preview", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Rule  Rule         `json:"rule"`
			Event PreviewEvent `json:"event"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Event.Kind) == 0 {
			write(w, http.StatusBadRequest, map[string]string{"error": "event.kind is required"})
			return
		}
		write(w, http.StatusOK, PreviewResult{Matched: true, Entities: []PreviewEntity{
			{ID: "urn:ngsi-ld:Room:r1", Type: "Room", Properties: map[string]any{}, Operation: "merge"},
		}, Observations: 1})
	})
	return httptest.NewServer(mux)
}

func testService(t *testing.T) (*Service, *map[string]string, func()) {
	t.Helper()
	seen := &map[string]string{}
	srv := stubTransform(t, seen)
	svc := NewService(&client.Client{}, srv.URL)
	return svc, seen, srv.Close
}

func TestListModelsParsesEnvelopeAndFilters(t *testing.T) {
	is := is.New(t)
	svc, seen, done := testService(t)
	defer done()

	models, err := svc.ListModels(context.Background(), "t", nil)
	is.NoErr(err)
	is.Equal(1, len(models))
	is.Equal("Room", models[0].Rule.Entities[0].Type)

	models, err = svc.ListModels(context.Background(), "t", map[string]string{"kind": "thing"})
	is.NoErr(err)
	is.Equal(1, len(models))
	is.True((*seen)["query"] != "")

	models, err = svc.ListModels(context.Background(), "t", map[string]string{"kind": "nope"})
	is.NoErr(err)
	is.Equal(0, len(models))
}

func TestGetModel(t *testing.T) {
	is := is.New(t)
	svc, _, done := testService(t)
	defer done()

	m, err := svc.GetModel(context.Background(), "t", "11111111-1111-1111-1111-111111111111")
	is.NoErr(err)
	is.Equal(int64(1), m.Revision)

	_, err = svc.GetModel(context.Background(), "t", "00000000-0000-0000-0000-000000000000")
	is.True(errors.Is(err, client.ErrNotFound))
}

func TestCreateModelValidationError(t *testing.T) {
	is := is.New(t)
	svc, _, done := testService(t)
	defer done()

	m, err := svc.CreateModel(context.Background(), "t", testRule())
	is.NoErr(err)
	is.Equal("api", m.Source)

	_, err = svc.CreateModel(context.Background(), "t", Rule{})
	var verr *ValidationError
	is.True(errors.As(err, &verr))
}

func TestWriteMapsForbidden(t *testing.T) {
	is := is.New(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	svc := NewService(&client.Client{}, srv.URL)
	_, err := svc.CreateModel(context.Background(), "t", testRule())
	is.True(errors.Is(err, client.ErrForbidden))
	is.True(svc.ValidateRule(context.Background(), "t", testRule()) != nil)
}

func TestUpdateModelConflictAndIfMatch(t *testing.T) {
	is := is.New(t)
	svc, seen, done := testService(t)
	defer done()

	id := "11111111-1111-1111-1111-111111111111"
	m, err := svc.UpdateModel(context.Background(), "t", id, testRule(), 1)
	is.NoErr(err)
	is.Equal(int64(2), m.Revision)
	is.Equal(`"rev-1"`, (*seen)["ifmatch"])

	_, err = svc.UpdateModel(context.Background(), "t", id, testRule(), 9)
	is.True(errors.Is(err, client.ErrConflict))
}

func TestDeleteModel(t *testing.T) {
	is := is.New(t)
	svc, _, done := testService(t)
	defer done()

	id := "11111111-1111-1111-1111-111111111111"
	is.NoErr(svc.DeleteModel(context.Background(), "t", id, 1))

	err := svc.DeleteModel(context.Background(), "t", "00000000-0000-0000-0000-000000000000", 1)
	is.True(errors.Is(err, client.ErrNotFound))

	err = svc.DeleteModel(context.Background(), "t", id, 9)
	is.True(errors.Is(err, client.ErrConflict))
}

func TestValidateAndPreview(t *testing.T) {
	is := is.New(t)
	svc, _, done := testService(t)
	defer done()

	is.NoErr(svc.ValidateRule(context.Background(), "t", testRule()))

	err := svc.ValidateRule(context.Background(), "t", Rule{})
	var verr *ValidationError
	is.True(errors.As(err, &verr))
	is.True(verr.Message != "")

	res, err := svc.PreviewRule(context.Background(), "t", testRule(), PreviewEvent{Kind: "things.v1.values", Message: map[string]any{}})
	is.NoErr(err)
	is.True(res.Matched)
	is.Equal(1, len(res.Entities))
	is.Equal("merge", res.Entities[0].Operation)
	is.Equal(1, res.Observations)

	_, err = svc.PreviewRule(context.Background(), "t", testRule(), PreviewEvent{})
	is.True(errors.As(err, &verr))
}
