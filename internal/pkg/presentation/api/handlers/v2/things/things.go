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
	"github.com/diwise/diwise-web/internal/application/admin"
	"github.com/diwise/diwise-web/internal/application/client"
	appthings "github.com/diwise/diwise-web/internal/application/things"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	featuresthings "github.com/diwise/diwise-web/internal/pkg/presentation/webv2/components/features/things"
	v2layout "github.com/diwise/diwise-web/internal/pkg/presentation/webv2/components/layout"
	"github.com/google/uuid"

	. "github.com/diwise/frontend-toolkit"
)

type thingsApp interface {
	admin.Management
	appthings.Management
}

func NewThingsPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app thingsApp) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "things",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		model, err := composeListModel(ctx, r, localizer, app)
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

func NewThingsDataList(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app thingsApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "things",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		model, err := composeListModel(ctx, r, localizer, app)
		if err != nil {
			http.Error(w, "could not fetch things", http.StatusInternalServerError)
			return
		}

		component := featuresthings.ThingsDataList(localizer, model)
		helpers.WriteComponentResponse(ctx, w, r, component, 16*1024, 0)
	}
}

func NewThingComponentHandler(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app thingsApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))
		model, err := composeNewThingModel(r.Context(), localizer, app)
		if err != nil {
			http.Error(w, "could not load new thing form", http.StatusInternalServerError)
			return
		}

		component := featuresthings.NewThingModal(localizer, model)
		helpers.WriteComponentResponse(r.Context(), w, r, component, 16*1024, 0)
	}
}

func NewCreateThingPage(_ context.Context, _ LocaleBundle, _ AssetLoaderFunc, app thingsApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "could not parse form data", http.StatusBadRequest)
			return
		}

		newThing := newThingFromForm(r.Form)
		err := app.NewThing(r.Context(), newThing)
		if err != nil {
			http.Error(w, "could not create new thing", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/v2/things/"+newThing.ID+"?mode=edit", http.StatusFound)
	}
}

func composeListModel(ctx context.Context, r *http.Request, localizer Localizer, app thingsApp) (featuresthings.ThingsPageViewModel, error) {
	pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
	offset, limit := helpers.GetOffsetAndLimit(r)
	showMap := r.URL.Query().Get("mapview") == "true"

	args := r.URL.Query()
	helpers.SanitizeParams(args, "mapview", "page", "limit", "offset")
	selectedTypes := normalizeTypeFilter(args)
	selectedTags := normalizeMultiValueFilter(args, "tags")

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
		label := localizer.Get(thingType)
		typeOptions = append(typeOptions, featuresthings.TypeOption{
			Value: thingType,
			Label: label,
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
			SelectedTypes: selectedTypes,
			SelectedTags:  selectedTags,
			PageSize:      limit,
		},
		TypeOptions:   typeOptions,
		TagOptions:    tagOptions,
		Organisations: app.GetTenants(ctx),
		MapView:       showMap,
	}

	for _, thing := range result.Things {
		model.Things = append(model.Things, toViewModel(thing))
	}

	return model, nil
}

func composeNewThingModel(ctx context.Context, localizer Localizer, app thingsApp) (featuresthings.NewThingViewModel, error) {
	types, err := app.GetTypes(ctx)
	if err != nil {
		return featuresthings.NewThingViewModel{}, err
	}

	typeOptions := make([]featuresthings.TypeOption, 0, len(types))
	for _, thingType := range types {
		typeOptions = append(typeOptions, featuresthings.TypeOption{
			Value: thingType,
			Label: localizer.Get(thingType),
		})
	}
	slices.SortFunc(typeOptions, func(a, b featuresthings.TypeOption) int {
		return cmp.Compare(a.Label, b.Label)
	})

	organisations := app.GetTenants(ctx)
	slices.Sort(organisations)

	return featuresthings.NewThingViewModel{
		TypeOptions:   typeOptions,
		Organisations: organisations,
	}, nil
}

func newThingFromForm(form url.Values) appthings.Thing {
	id := uuid.NewString()
	thingType := strings.TrimSpace(form.Get("type"))
	thingSubType := ""
	switch {
	case strings.Contains(thingType, ":"):
		parts := strings.SplitN(thingType, ":", 2)
		thingType = parts[0]
		thingSubType = parts[1]
	case strings.Contains(thingType, "-"):
		parts := strings.SplitN(thingType, "-", 2)
		thingType = parts[0]
		thingSubType = parts[1]
	}

	return appthings.Thing{
		ID:          id,
		Type:        thingType,
		SubType:     thingSubType,
		Name:        strings.TrimSpace(form.Get("name")),
		Description: strings.TrimSpace(form.Get("description")),
		Location: client.Location{
			Latitude:  0,
			Longitude: 0,
		},
		Tenant: strings.TrimSpace(form.Get("organisation")),
	}
}

func normalizeTypeFilter(args url.Values) []string {
	return normalizeMultiValueFilter(args, "type")
}

func normalizeMultiValueFilter(args url.Values, key string) []string {
	rawValues := args[key]
	if len(rawValues) == 0 {
		return nil
	}

	selectedValues := make([]string, 0, len(rawValues))
	for _, rawValue := range rawValues {
		for _, part := range strings.Split(rawValue, ",") {
			part = strings.TrimSpace(part)
			if part == "" || slices.Contains(selectedValues, part) {
				continue
			}
			selectedValues = append(selectedValues, part)
		}
	}

	args.Del(key)
	for _, value := range selectedValues {
		args.Add(key, value)
	}

	return selectedValues
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

func toViewModel(thing appthings.Thing) featuresthings.ThingViewModel {
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
