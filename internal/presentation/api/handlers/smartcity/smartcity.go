package smartcity

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	appsmartcity "github.com/diwise/diwise-web/internal/application/smartcity"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	featuresmartcity "github.com/diwise/diwise-web/internal/presentation/web/components/features/smartcity"
	v2layout "github.com/diwise/diwise-web/internal/presentation/web/components/layout"
	shared "github.com/diwise/diwise-web/internal/presentation/web/components/shared"

	. "github.com/diwise/frontend-toolkit"
)

const panelTarget = "#smart-city-panel"

type smartCityApp interface {
	GetSmartCityObjects(ctx context.Context, types []string) ([]appsmartcity.Object, error)
	GetSmartCityObject(ctx context.Context, id string) (appsmartcity.Object, error)
	CreateSmartCityObject(ctx context.Context, object appsmartcity.Object) error
	SmartCityTypes() []string
}

func NewSmartCityPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app smartCityApp) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "smartcity",
		)
		ctx = shared.WithMarkerLinkTarget(ctx, panelTarget)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		model, err := composeModel(ctx, localizer, app)
		if err != nil {
			http.Error(w, "could not fetch objects", http.StatusInternalServerError)
			return
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

func NewSmartCityObjectPanel(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app smartCityApp) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		object, err := app.GetSmartCityObject(r.Context(), r.PathValue("id"))
		if err != nil {
			http.Error(w, "could not fetch object", http.StatusNotFound)
			return
		}

		viewModel := toViewModel(object)
		helpers.WriteComponentResponse(r.Context(), w, r, featuresmartcity.ObjectPanel(localizer, &viewModel), 8*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewCreateSmartCityObject(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app smartCityApp) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		model := featuresmartcity.PageViewModel{Types: composeTypes(localizer, app)}
		helpers.WriteComponentResponse(r.Context(), w, r, featuresmartcity.CreateObjectModal(localizer, model), 16*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewCreateSmartCityObjectPost(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app smartCityApp) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "could not parse form data", http.StatusBadRequest)
			return
		}

		object := appsmartcity.Object{
			ID:   strings.TrimSpace(r.Form.Get("id")),
			Type: strings.TrimSpace(r.Form.Get("type")),
			Name: strings.TrimSpace(r.Form.Get("name")),
		}

		if object.ID == "" || object.Type == "" {
			http.Error(w, "id and type are required", http.StatusBadRequest)
			return
		}

		if lat, err := strconv.ParseFloat(strings.TrimSpace(r.Form.Get("latitude")), 64); err == nil {
			if lon, err := strconv.ParseFloat(strings.TrimSpace(r.Form.Get("longitude")), 64); err == nil {
				object.Latitude, object.Longitude, object.HasLocation = lat, lon, true
			}
		}

		if err := app.CreateSmartCityObject(r.Context(), object); err != nil {
			http.Error(w, "could not create object", http.StatusInternalServerError)
			return
		}

		ctx := shared.WithMarkerLinkTarget(r.Context(), panelTarget)
		localizer := l10n.For(r.Header.Get("Accept-Language"))
		model, err := composeModel(ctx, localizer, app)
		if err != nil {
			http.Error(w, "could not fetch objects", http.StatusInternalServerError)
			return
		}

		helpers.WriteComponentResponse(ctx, w, r, featuresmartcity.SmartCityContent(localizer, model), 32*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func composeModel(ctx context.Context, localizer Localizer, app smartCityApp) (featuresmartcity.PageViewModel, error) {
	objects, err := app.GetSmartCityObjects(ctx, nil)
	if err != nil {
		return featuresmartcity.PageViewModel{}, err
	}

	model := featuresmartcity.PageViewModel{
		Objects: make([]featuresmartcity.ObjectViewModel, 0, len(objects)),
		Types:   composeTypes(localizer, app),
	}

	for _, object := range objects {
		model.Objects = append(model.Objects, toViewModel(object))
	}

	return model, nil
}

func composeTypes(localizer Localizer, app smartCityApp) []featuresmartcity.TypeOption {
	known := app.SmartCityTypes()
	options := make([]featuresmartcity.TypeOption, 0, len(known))
	for _, t := range known {
		label := localizer.Get(strings.ToLower(t))
		if label == "" {
			label = t
		}
		options = append(options, featuresmartcity.TypeOption{Value: t, Label: label})
	}
	return options
}

func toViewModel(object appsmartcity.Object) featuresmartcity.ObjectViewModel {
	return featuresmartcity.ObjectViewModel{
		ID:          object.ID,
		Type:        object.Type,
		Name:        object.Name,
		Latitude:    object.Latitude,
		Longitude:   object.Longitude,
		HasLocation: object.HasLocation,
	}
}
