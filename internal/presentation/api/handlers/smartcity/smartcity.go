package smartcity

import (
	"context"
	"net/http"

	appsmartcity "github.com/diwise/diwise-web/internal/application/smartcity"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	featuresmartcity "github.com/diwise/diwise-web/internal/presentation/web/components/features/smartcity"
	v2layout "github.com/diwise/diwise-web/internal/presentation/web/components/layout"

	"github.com/a-h/templ"
	. "github.com/diwise/frontend-toolkit"
)

type smartCityApp interface {
	GetSmartCityObjects(ctx context.Context) ([]appsmartcity.Object, error)
}

func NewSmartCityPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app smartCityApp) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "smartcity",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		objects, err := app.GetSmartCityObjects(ctx)
		if err != nil {
			http.Error(w, "could not fetch city objects", http.StatusInternalServerError)
			return
		}

		model := featuresmartcity.PageViewModel{
			Objects:     toViewModels(objects),
			Icons:       composeIcons(assets),
			DefaultIcon: assets("/markers/thing.svg").Path(),
		}

		content := featuresmartcity.SmartCityPage(localizer, model)
		page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
		if helpers.IsHxRequest(r) {
			page = v2layout.AppShell(localizer, assets, content)
		}

		helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, 0)
	}

	return http.HandlerFunc(fn)
}

// markörfil per entitetstyp (gemener). Okända typer får DefaultIcon.
var markerFiles = map[string]string{
	"building":             "building",
	"beach":                "map-pin",
	"pointofinterest":      "map-pin",
	"lifebuoy":             "lifebuoy",
	"room":                 "thermometer",
	"wastecontainer":       "wastecontainer",
	"sewagepumpingstation": "sewer",
	"greenspacerecord":     "sandpocket",
	"device":               "sensor",
	"desk":                 "desk",
	"passage":              "door",
	"watermeter":           "water",
}

func composeIcons(assets AssetLoaderFunc) map[string]string {
	icons := make(map[string]string, len(markerFiles))
	for entityType, file := range markerFiles {
		icons[entityType] = assets("/markers/" + file + ".svg").Path()
	}
	return icons
}

func toViewModels(objects []appsmartcity.Object) []featuresmartcity.ObjectViewModel {
	viewModels := make([]featuresmartcity.ObjectViewModel, 0, len(objects))
	for _, object := range objects {
		viewModels = append(viewModels, featuresmartcity.ObjectViewModel{
			ID:          object.ID,
			Type:        object.Type,
			Name:        object.Name,
			Tenant:      object.Tenant,
			Latitude:    object.Latitude,
			Longitude:   object.Longitude,
			HasLocation: object.HasLocation,
		})
	}
	return viewModels
}
