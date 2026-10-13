package thingsv2

import (
	"context"
	"strings"
	"testing"

	"github.com/matryer/is"
)

func testCreateGeometryModel() ThingV2CreateViewModel {
	return ThingV2CreateViewModel{
		Tenants:         []string{"t1"},
		Tenant:          "t1",
		Template:        "room/v2",
		TemplateDisplay: "Rum v2",
		GeometryKinds:   []string{"Point", "Polygon"},
		GeometryMode:    "point",
		AllowNoLocation: true,
	}
}

func TestCreatePageRendersGeometryModes(t *testing.T) {
	is := is.New(t)

	var sb strings.Builder
	is.NoErr(ThingV2CreatePage(testLocalizer(), testCreateGeometryModel()).Render(context.Background(), &sb))
	html := sb.String()

	is.True(strings.Contains(html, `name="geometryMode"`))
	is.True(strings.Contains(html, `value="point"`))
	is.True(strings.Contains(html, `value="polygon"`))
	is.True(strings.Contains(html, `value="none"`))
	is.True(strings.Contains(html, `id="geometry" name="geometry"`))
	is.True(strings.Contains(html, `data-geometry-mode="point"`))
	is.True(strings.Contains(html, `id="location-point-fields"`))
	is.True(strings.Contains(html, `id="location-polygon-tools"`))
	is.True(strings.Contains(html, `id="geometry-undo"`))
	is.True(strings.Contains(html, `id="geometry-clear"`))
	// Punktläge: punktfält synliga, polygontools dolda.
	is.True(!strings.Contains(html, `id="location-point-fields" class="grid gap-4 md:grid-cols-2" hidden`))
	is.True(strings.Contains(html, `id="location-polygon-tools" class="flex min-w-0 flex-col gap-2" hidden`))
}

func TestCreatePagePreselectsNoneMode(t *testing.T) {
	is := is.New(t)

	model := testCreateGeometryModel()
	model.GeometryMode = "none"

	var sb strings.Builder
	is.NoErr(ThingV2CreatePage(testLocalizer(), model).Render(context.Background(), &sb))
	html := sb.String()

	is.True(strings.Contains(html, `data-geometry-mode="none"`))
	is.True(strings.Contains(html, `value="none"`))
}

func TestEditPageRendersGeometryMode(t *testing.T) {
	is := is.New(t)

	model := ThingV2EditViewModel{
		ThingID:         "room-1",
		Tenant:          "t1",
		Revision:        3,
		TemplateDisplay: "Rum v2",
		Name:            "Rum",
		GeometryKinds:   []string{"Point", "Polygon"},
		GeometryMode:    "polygon",
		GeometryJSON:    `[[[17.3,62.39],[17.4,62.39]]]`,
		AllowNoLocation: true,
	}

	var sb strings.Builder
	is.NoErr(ThingV2EditPage(testLocalizer(), model).Render(context.Background(), &sb))
	html := sb.String()

	is.True(strings.Contains(html, `data-geometry-mode="polygon"`))
	is.True(strings.Contains(html, `name="geometry"`))
	// Polygonläge: punktfält dolda, polygontools synliga.
	is.True(strings.Contains(html, `id="location-point-fields" class="grid gap-4 md:grid-cols-2" hidden`))
	is.True(!strings.Contains(html, `id="location-polygon-tools" class="flex min-w-0 flex-col gap-2" hidden`))
}
