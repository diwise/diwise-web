package things

import (
	"cmp"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	featuresthings "github.com/diwise/diwise-web/internal/pkg/presentation/webv2/components/features/things"
	v2layout "github.com/diwise/diwise-web/internal/pkg/presentation/webv2/components/layout"

	. "github.com/diwise/frontend-toolkit"
)

func NewThingsPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "things",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		model, err := composeListModel(ctx, r, app)
		if err != nil {
			http.Error(w, "could not fetch things", http.StatusInternalServerError)
			return
		}

		content := featuresthings.ThingsPage(localizer, model)
		page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
		if helpers.IsHxRequest(r) {
			page = v2layout.AppShell(localizer, assets, content)
		}

		helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, 0)
	}
}

func NewThingsDataList(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "things",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		model, err := composeListModel(ctx, r, app)
		if err != nil {
			http.Error(w, "could not fetch things", http.StatusInternalServerError)
			return
		}

		component := featuresthings.ThingsDataList(localizer, model)
		helpers.WriteComponentResponse(ctx, w, r, component, 16*1024, 0)
	}
}

func composeListModel(ctx context.Context, r *http.Request, app application.ThingManagement) (featuresthings.ThingsPageViewModel, error) {
	pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
	offset, limit := helpers.GetOffsetAndLimit(r)
	showMap := r.URL.Query().Get("mapview") == "true"

	args := r.URL.Query()
	helpers.SanitizeParams(args, "mapview", "page", "limit", "offset")

	if showMap {
		offset = 0
		limit = 1000
	}

	result, err := app.GetThings(ctx, offset, limit, args)
	if err != nil {
		return featuresthings.ThingsPageViewModel{}, err
	}

	tags, err := app.GetTags(ctx)
	if err != nil {
		return featuresthings.ThingsPageViewModel{}, err
	}

	types, err := app.GetTypes(ctx)
	if err != nil {
		return featuresthings.ThingsPageViewModel{}, err
	}

	typeOptions := make([]featuresthings.TypeOption, 0, len(types))
	for _, thingType := range types {
		typeOptions = append(typeOptions, featuresthings.TypeOption{
			Value: thingType,
			Label: thingType,
		})
	}
	slices.SortFunc(typeOptions, func(a, b featuresthings.TypeOption) int {
		return cmp.Compare(a.Label, b.Label)
	})

	tagOptions := make([]featuresthings.TagOption, 0, len(tags))
	for _, tag := range tags {
		tagOptions = append(tagOptions, featuresthings.TagOption{Value: tag})
	}
	slices.SortFunc(tagOptions, func(a, b featuresthings.TagOption) int {
		return cmp.Compare(a.Value, b.Value)
	})

	pageIndexInt, _ := strconv.Atoi(pageIndex)
	pageLast := int(math.Ceil(float64(result.TotalRecords) / float64(limit)))

	model := featuresthings.ThingsPageViewModel{
		Things: make([]featuresthings.ThingViewModel, 0, len(result.Things)),
		Paging: featuresthings.PagingViewModel{
			PageIndex:  max(pageIndexInt, 1),
			PageLast:   max(pageLast, 1),
			PageSize:   limit,
			TotalCount: result.TotalRecords,
			Query:      args.Encode(),
			TargetURL:  "/v2/components/things/list",
			TargetID:   "#tableOrMap",
		},
		Filters: featuresthings.FiltersViewModel{
			SelectedTypes: selectedValues(r.URL.Query(), "type"),
			SelectedTags:  selectedValues(r.URL.Query(), "tags"),
			PageSize:      limit,
		},
		TypeOptions: typeOptions,
		TagOptions:  tagOptions,
		MapView:     showMap,
	}

	for _, thing := range result.Things {
		model.Things = append(model.Things, toViewModel(thing))
	}

	return model, nil
}

func selectedValues(values url.Values, key string) []string {
	raw := values[key]
	if len(raw) == 0 {
		return nil
	}

	result := make([]string, 0, len(raw))
	for _, item := range raw {
		for _, part := range strings.Split(item, ",") {
			part = strings.TrimSpace(part)
			if part == "" || slices.Contains(result, part) {
				continue
			}
			result = append(result, part)
		}
	}

	return result
}

func toViewModel(thing application.Thing) featuresthings.ThingViewModel {
	viewModel := featuresthings.ThingViewModel{
		ID:              thing.ID,
		Type:            thing.Type,
		SubType:         thing.SubType,
		Name:            thing.Name,
		AlternativeName: thing.AlternativeName,
		Description:     thing.Description,
		Latitude:        thing.Location.Latitude,
		Longitude:       thing.Location.Longitude,
		RefDevice:       make([]string, 0, len(thing.RefDevices)),
		Tenant:          thing.Tenant,
		Tags:            thing.Tags,
		ObservedAt:      thing.ObservedAt,
		Measurements:    make([]featuresthings.MeasurementViewModel, 0),
		Latest:          make(map[string]featuresthings.MeasurementViewModel),
		Properties:      make(map[string]any),
	}

	for _, ref := range thing.RefDevices {
		viewModel.RefDevice = append(viewModel.RefDevice, ref.DeviceID)
	}

	if len(viewModel.RefDevice) == 0 {
		viewModel.RefDevice = append(viewModel.RefDevice, "")
	}

	if len(thing.Values) > 0 {
		for _, measurement := range thing.Values[0] {
			stringValue := ""
			if measurement.StringValue != nil {
				stringValue = *measurement.StringValue
			}

			viewModel.Measurements = append(viewModel.Measurements, featuresthings.MeasurementViewModel{
				ID:          measurement.ID,
				Timestamp:   measurement.Timestamp,
				Urn:         measurement.Urn,
				BoolValue:   measurement.BoolValue,
				StringValue: stringValue,
				Unit:        measurement.Unit,
				Value:       measurement.Value,
			})
		}
	}

	for key, value := range toMap(thing.TypeValues) {
		if value != nil {
			viewModel.Properties[key] = value
		}
	}

	return viewModel
}

func toMap(v any) map[string]any {
	data, _ := json.Marshal(v)
	values := map[string]any{}
	_ = json.Unmarshal(data, &values)
	return values
}
