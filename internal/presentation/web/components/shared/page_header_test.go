package shared

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/matryer/is"
)

// Detaljrubriken ska vara samma struktur i alla domäner: baklänk, titel,
// metarad, framhävt värde och åtgärder.
func TestPageHeaderRendersAllSlots(t *testing.T) {
	is := is.New(t)

	component := PageHeader(PageHeaderProps{
		BackHref:  "/things",
		BackLabel: "Saker",
		Title:     "Soptunna 42",
		Meta:      templ.Raw(`<span class="chip">container</span>`),
		Primary:   templ.Raw(`<span>42 %</span>`),
		Actions:   templ.Raw(`<button type="button">Redigera</button>`),
	})

	var sb strings.Builder
	is.NoErr(component.Render(context.Background(), &sb))
	html := sb.String()

	is.True(strings.Contains(html, `href="/things"`))
	is.True(strings.Contains(html, `hx-target="#app-shell"`))
	is.True(strings.Contains(html, `Saker`))
	is.True(strings.Contains(html, `<h1`))
	is.True(strings.Contains(html, `Soptunna 42`))
	is.True(strings.Contains(html, `class="chip"`))
	is.True(strings.Contains(html, `42 %`))
	is.True(strings.Contains(html, `Redigera`))
	// Rubriken är ett kort med samma ram/radie som övriga kort.
	is.True(strings.Contains(html, "rounded-2xl border border-border"))
	is.True(strings.Contains(html, "sm:text-3xl"))
}

// FormPageHeader är den lättare varianten utan kortram och utan åtgärder.
func TestFormPageHeaderHasNoCardFrame(t *testing.T) {
	is := is.New(t)

	var sb strings.Builder
	is.NoErr(FormPageHeader(FormPageHeaderProps{
		BackHref:  "/things-v2",
		BackLabel: "Saker v2",
		Title:     "Ny sak",
	}).Render(context.Background(), &sb))
	html := sb.String()

	is.True(strings.Contains(html, `href="/things-v2"`))
	is.True(strings.Contains(html, `Ny sak`))
	is.True(!strings.Contains(html, "rounded-2xl border border-border"))
}
