package shared

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func TestListFiltersCardRendersMultipleActionsWithoutToggle(t *testing.T) {
	var html strings.Builder
	err := ListFiltersCard(nil,
		templ.Raw(`<button>Exportera</button>`),
		nil,
		templ.Raw(`<button>Skapa</button>`),
	).Render(context.Background(), &html)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"Exportera", "Skapa"} {
		if !strings.Contains(html.String(), action) {
			t.Errorf("missing filter action %q", action)
		}
	}
}
