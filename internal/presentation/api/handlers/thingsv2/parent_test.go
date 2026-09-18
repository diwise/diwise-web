package thingsv2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matryer/is"
)

func TestThingsV2ParentDialogRendersSearch(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2ParentDialog(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/components/things-v2/bin-1/parent-dialog?tenant=t1", nil)
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	is.True(strings.Contains(body, "thing-v2-parent-dialog"))
	is.True(strings.Contains(body, "Tunna"))
	is.True(strings.Contains(body, "thing-v2-parent-search"))
	is.True(strings.Contains(body, "revision=2"))
}

func TestThingsV2ParentSearchExcludesSelf(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2ParentSearch(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/components/things-v2/bin-1/parents?tenant=t1&query=t&revision=2", nil)
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	// Stubblistan innehåller Tunna (bin-1) + Område (area-1); saken
	// själv ska inte erbjudas som förälder.
	is.True(strings.Contains(body, "Område"))
	is.True(!strings.Contains(body, "Tunna"))
	// Välj är ett formulär med submit (fungerar även utan htmx-metoder).
	is.True(strings.Contains(body, "<form"))
	is.True(strings.Contains(body, `name="parentId"`))
	is.True(strings.Contains(body, `type="submit"`))
}

func TestThingsV2ParentSearchEmptyQueryRendersNothing(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2ParentSearch(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/components/things-v2/bin-1/parents?tenant=t1&revision=2", nil)
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.True(!strings.Contains(rec.Body.String(), "area-1"))
}

func TestThingsV2SetParentRedirectsToDetails(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2SetParent(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodPost, "/components/things-v2/bin-1/parent?tenant=t1", strings.NewReader("parentId=gh-1&revision=2"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.Equal("/things-v2/bin-1?tenant=t1", rec.Header().Get("HX-Redirect"))
}

func TestThingsV2SetParentRejectsSelf(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2SetParent(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodPost, "/components/things-v2/bin-1/parent?tenant=t1", strings.NewReader("parentId=bin-1&revision=2"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusBadRequest, rec.Code)
}

func TestThingsV2UnlinkParentRedirectsToDetails(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2UnlinkParent(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodPost, "/components/things-v2/bin-1/parent/remove?tenant=t1", strings.NewReader("revision=2"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.Equal("/things-v2/bin-1?tenant=t1", rec.Header().Get("HX-Redirect"))
}

func TestThingsV2DetailsHidesParentButtonWithoutSlot(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	// bin-1 är wastebin utan partOf-slot: ingen byt-knapp ska renderas.
	handler := NewThingsV2DetailsPage(context.Background(), testLocaleBundle(), testAssets(), &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/things-v2/bin-1?tenant=t1", nil)
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.True(!strings.Contains(rec.Body.String(), "parent-dialog"))
}

func TestThingsV2ParentSearchFiltersAllowedTemplates(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2ParentSearch(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	// tank-1 har partOf → [greenhouse]: bara växthuset ska hittas,
	// trots att stubblistan även har tunna och område.
	req := httptest.NewRequest(http.MethodGet, "/components/things-v2/tank-1/parents?tenant=t1&query=a&revision=1", nil)
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "tank-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	is.True(strings.Contains(body, "Växthus"))
	is.True(!strings.Contains(body, "Tunna"))
	is.True(!strings.Contains(body, "Område"))
}

func TestPartOfSlotReportsMissingSlot(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()
	app := &testThingsV2App{svc: svc}

	_, ok, err := partOfSlot(context.Background(), app, "t1", "bin-1")
	is.NoErr(err)
	is.True(!ok)

	allowed, ok, err := partOfSlot(context.Background(), app, "t1", "tank-1")
	is.NoErr(err)
	is.True(ok)
	is.Equal([]string{"greenhouse"}, allowed)
}
