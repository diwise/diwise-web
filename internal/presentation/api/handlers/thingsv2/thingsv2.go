package thingsv2

import (
	"cmp"
	"context"
	"errors"
	"math"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/application/client"
	appthingsv2 "github.com/diwise/diwise-web/internal/application/thingsv2"
	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	featuresthings "github.com/diwise/diwise-web/internal/presentation/web/components/features/things"
	featuresthingsv2 "github.com/diwise/diwise-web/internal/presentation/web/components/features/thingsv2"
	v2layout "github.com/diwise/diwise-web/internal/presentation/web/components/layout"
	shared "github.com/diwise/diwise-web/internal/presentation/web/components/shared"

	. "github.com/diwise/frontend-toolkit"
)

type thingsV2App interface {
	ThingsV2() *appthingsv2.Service
}

func NewThingsV2Page(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app thingsV2App) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "things-v2",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		model, err := composeListModel(ctx, r, localizer, app)
		if err != nil {
			http.Error(w, "could not fetch things", http.StatusInternalServerError)
			return
		}

		content := featuresthingsv2.ThingsV2Page(localizer, model)
		page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
		if helpers.IsHxRequest(r) {
			page = v2layout.AppShell(localizer, assets, content)
		}

		helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewThingsV2DataList(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app thingsV2App) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "things-v2",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		model, err := composeListModel(ctx, r, localizer, app)
		if err != nil {
			http.Error(w, "could not fetch things", http.StatusInternalServerError)
			return
		}

		component := featuresthingsv2.ThingsV2DataList(localizer, model)
		helpers.WriteComponentResponse(ctx, w, r, component, 16*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func composeListModel(ctx context.Context, r *http.Request, localizer Localizer, app thingsV2App) (featuresthingsv2.ThingsV2PageViewModel, error) {
	pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
	offset, limit := helpers.GetOffsetAndLimit(r)
	showMap := r.URL.Query().Get("mapview") == "true"

	args := r.URL.Query()
	helpers.SanitizeParams(args, "mapview", "page", "limit", "offset")
	selectedCategories := selectedValues(args, "category")
	selectedTemplates := selectedValues(args, "template")
	nameFilter := args.Get("name")

	if showMap {
		offset = 0
		limit = 1000
	}

	filter := appthingsv2.Filter{Limit: limit, Offset: offset}
	if len(selectedCategories) > 0 {
		filter.Category = selectedCategories[0]
	}
	if len(selectedTemplates) > 0 {
		filter.Template = selectedTemplates[0]
	}
	if nameFilter != "" {
		filter.Name = nameFilter
	}

	// Servern fanar ut över token-auktoriserade tenants; inget tenantval här.
	result, err := app.ThingsV2().ListThingsAcrossTenants(ctx, filter)
	if err != nil {
		return featuresthingsv2.ThingsV2PageViewModel{}, err
	}

	templates, err := app.ThingsV2().ListTemplates(ctx, "", "")
	if err != nil {
		return featuresthingsv2.ThingsV2PageViewModel{}, err
	}

	categoryOptions := make([]featuresthings.TypeOption, 0)
	seenCategories := map[string]bool{}
	templateOptions := make([]featuresthings.TypeOption, 0, len(templates))
	for _, spec := range templates {
		label := spec.Template.DisplayName
		if label == "" {
			label = spec.Template.ID
		}
		templateOptions = append(templateOptions, featuresthings.TypeOption{
			Value: spec.Template.ID,
			Label: label,
		})
		if spec.Template.Category != "" && !seenCategories[spec.Template.Category] {
			seenCategories[spec.Template.Category] = true
			categoryOptions = append(categoryOptions, featuresthings.TypeOption{
				Value: spec.Template.Category,
				Label: spec.Template.Category,
			})
		}
	}
	slices.SortFunc(templateOptions, func(a, b featuresthings.TypeOption) int {
		return cmp.Compare(a.Label, b.Label)
	})
	slices.SortFunc(categoryOptions, func(a, b featuresthings.TypeOption) int {
		return cmp.Compare(a.Label, b.Label)
	})

	pageIndexInt, _ := strconv.Atoi(pageIndex)
	pageLast := int(math.Ceil(float64(result.Total) / float64(limit)))

	model := featuresthingsv2.ThingsV2PageViewModel{
		Things: make([]featuresthingsv2.ThingV2ViewModel, 0, len(result.Things)),
		Paging: featuresthings.PagingViewModel{
			PageIndex:  max(pageIndexInt, 1),
			PageLast:   max(pageLast, 1),
			PageSize:   limit,
			TotalCount: result.Total,
			Query:      args.Encode(),
			TargetURL:  "/components/things-v2/list",
			TargetID:   "#tableOrMapV2",
		},
		Filters: featuresthingsv2.FiltersViewModel{
			SelectedCategories: selectedCategories,
			SelectedTemplates:  selectedTemplates,
			Name:               nameFilter,
			PageSize:           limit,
		},
		CategoryOptions: categoryOptions,
		TemplateOptions: templateOptions,
		MapView:         showMap,
	}

	for _, thing := range result.Things {
		model.Things = append(model.Things, toViewModel(thing))
	}

	return model, nil
}

func selectedValues(values map[string][]string, key string) []string {
	raw := values[key]
	if len(raw) == 0 {
		return nil
	}

	result := make([]string, 0, len(raw))
	for _, item := range raw {
		for part := range strings.SplitSeq(item, ",") {
			part = strings.TrimSpace(part)
			if part == "" || slices.Contains(result, part) {
				continue
			}
			result = append(result, part)
		}
	}

	return result
}

func toViewModel(thing appthingsv2.Thing) featuresthingsv2.ThingV2ViewModel {
	viewModel := featuresthingsv2.ThingV2ViewModel{
		ID:         thing.ThingID,
		Tenant:     thing.Tenant,
		Name:       thing.Name,
		Category:   thing.Category,
		TemplateID: thing.TemplateID,
	}

	if lon, lat, ok := thing.Location.Point(); ok {
		viewModel.HasLocation = true
		viewModel.Longitude = lon
		viewModel.Latitude = lat
	} else if thing.Location != nil {
		viewModel.HasGeometry = true
		viewModel.Geometry = thing.Location.Coordinates
		viewModel.GeometryType = thing.Location.Type
	}

	if thing.Primary != nil && thing.Primary.Value != nil {
		viewModel.PrimaryLabel = thing.Primary.DisplayName
		if viewModel.PrimaryLabel == "" {
			viewModel.PrimaryLabel = thing.Primary.PropertyID
		}
		viewModel.PrimaryValue = *thing.Primary.Value
		viewModel.HasPrimaryValue = true
		viewModel.PrimaryUnit = thing.Primary.Unit
		viewModel.PrimaryQuality = thing.Primary.Quality
	}

	return viewModel
}

func NewThingsV2DetailsPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app thingsV2App) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "things-v2",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))

		tenant, err := resolveDetailsTenant(r)
		if err != nil {
			http.Error(w, "tenant is required", http.StatusBadRequest)
			return
		}

		thing, err := app.ThingsV2().GetThing(ctx, tenant, id)
		if err != nil {
			if errors.Is(err, client.ErrNotFound) {
				http.Error(w, "thing not found", http.StatusNotFound)
				return
			}
			http.Error(w, "could not fetch thing", http.StatusInternalServerError)
			return
		}

		content := featuresthingsv2.ThingV2DetailsPage(localizer, toDetailsViewModel(thing))
		page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
		if helpers.IsHxRequest(r) {
			page = v2layout.AppShell(localizer, assets, content)
		}

		helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, 0)
	}

	return http.HandlerFunc(fn)
}

// resolveDetailsTenant väljer tenant för en sakläsning: explicit
// ?tenant= vinner, annars den enda token-auktoriserade tenanten.
func resolveDetailsTenant(r *http.Request) (string, error) {
	if tenant := strings.TrimSpace(r.URL.Query().Get("tenant")); tenant != "" {
		return tenant, nil
	}

	// Routen är redan skyddad med things.read; här räcker tokenens tenants.
	tenants := auth.GetTenantsWithAllowedScopes(r.Context(), auth.AnyScope)
	if len(tenants) == 1 {
		return tenants[0], nil
	}

	return "", errors.New("ambiguous tenant")
}

func toDetailsViewModel(thing appthingsv2.Thing) featuresthingsv2.ThingV2DetailsViewModel {
	model := featuresthingsv2.ThingV2DetailsViewModel{
		Thing:           toViewModel(thing),
		Values:          make([]featuresthingsv2.ThingV2ValueViewModel, 0, len(thing.Values)),
		Metadata:        make([]featuresthingsv2.MetadataItem, 0, len(thing.Metadata)),
		TemplateVersion: thing.TemplateVersion,
		VariantID:       thing.VariantID,
		VariantVersion:  thing.VariantVersion,
		Revision:        thing.Revision,
	}

	for id, value := range thing.Values {
		item := featuresthingsv2.ThingV2ValueViewModel{
			PropertyID: id,
			Label:      value.DisplayName,
			Unit:       value.Unit,
			Quality:    value.Quality,
			ObservedAt: value.ObservedAt,
		}
		if value.Value != nil {
			item.HasValue = true
			item.Value = *value.Value
		}
		model.Values = append(model.Values, item)
	}
	sort.Slice(model.Values, func(i, j int) bool {
		return model.Values[i].PropertyID < model.Values[j].PropertyID
	})

	for key, value := range thing.Metadata {
		model.Metadata = append(model.Metadata, featuresthingsv2.MetadataItem{Key: key, Value: value})
	}
	sort.Slice(model.Metadata, func(i, j int) bool {
		return model.Metadata[i].Key < model.Metadata[j].Key
	})

	model.Tenant = thing.Tenant
	for _, value := range model.Values {
		label := value.Label
		if strings.TrimSpace(label) == "" {
			label = value.PropertyID
		}
		model.HistoryProperties = append(model.HistoryProperties, featuresthings.TypeOption{
			Value: value.PropertyID,
			Label: label,
		})
	}
	if thing.Primary != nil && strings.TrimSpace(thing.Primary.PropertyID) != "" {
		model.DefaultHistoryProperty = thing.Primary.PropertyID
	} else if len(model.Values) > 0 {
		model.DefaultHistoryProperty = model.Values[0].PropertyID
	}

	return model
}

func NewThingsV2HistoryComponent(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app thingsV2App) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "things-v2",
		)

		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		tenant, err := resolveDetailsTenant(r)
		if err != nil {
			http.Error(w, "tenant is required", http.StatusBadRequest)
			return
		}

		property := strings.TrimSpace(r.URL.Query().Get("property"))
		if property == "" {
			http.Error(w, "property is required", http.StatusBadRequest)
			return
		}

		thing, err := app.ThingsV2().GetThing(ctx, tenant, id)
		if err != nil {
			if errors.Is(err, client.ErrNotFound) {
				http.Error(w, "thing not found", http.StatusNotFound)
				return
			}
			http.Error(w, "could not fetch thing", http.StatusInternalServerError)
			return
		}

		span := strings.TrimSpace(r.URL.Query().Get("span"))
		from, to := historySpanRange(span, time.Now().UTC())

		points, err := app.ThingsV2().GetHistory(ctx, tenant, id, property, from, to, 1000)
		if err != nil {
			http.Error(w, "could not fetch history", http.StatusInternalServerError)
			return
		}

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		model := toHistoryViewModel(r, localizer, thing, property, span, points)
		helpers.WriteComponentResponse(ctx, w, r, featuresthingsv2.ThingV2HistoryContent(localizer, model), 24*1024, 0)
	}

	return http.HandlerFunc(fn)
}

// historySpanRange mappar ett tidsspann till [from, to]: "today" är
// dygnets början (UTC) till nu och är förvalet på detaljsidan.
func historySpanRange(span string, now time.Time) (time.Time, time.Time) {
	switch span {
	case "24h":
		return now.Add(-24 * time.Hour), now
	case "7d":
		return now.Add(-7 * 24 * time.Hour), now
	case "30d":
		return now.Add(-30 * 24 * time.Hour), now
	default:
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		return startOfDay, now
	}
}

func toHistoryViewModel(r *http.Request, localizer Localizer, thing appthingsv2.Thing, property, span string, points []appthingsv2.HistoryPoint) featuresthingsv2.ThingV2HistoryViewModel {
	label := property
	unit := ""
	if current, ok := thing.Values[property]; ok {
		if strings.TrimSpace(current.DisplayName) != "" {
			label = current.DisplayName
		}
		unit = current.Unit
	}

	isDark := helpers.IsDarkMode(r)
	color := "#1F1F25"
	if isDark {
		color = "#FFFFFF"
	}

	model := featuresthingsv2.ThingV2HistoryViewModel{
		ThingID:       thing.ThingID,
		PropertyID:    property,
		PropertyLabel: label,
		Unit:          unit,
		Span:          span,
		Chart:         historyChartConfig(isDark, color, label, points),
		Points:        len(points),
	}

	for _, point := range points {
		if point.Value != nil {
			model.HasData = true
			break
		}
	}

	return model
}

func historyChartConfig(isDark bool, color, label string, points []appthingsv2.HistoryPoint) shared.AdvancedChartConfig {
	labels := make([]string, 0, len(points))
	data := make([]any, 0, len(points))
	for _, point := range points {
		labels = append(labels, point.ObservedAt.Format("2006-01-02 15:04"))
		if point.Value != nil {
			data = append(data, *point.Value)
		} else {
			data = append(data, nil)
		}
	}

	foreground := "#1F1F25"
	muted := "#444450"
	border := "#1F1F25"
	grid := "#E2E2E8"
	background := "#FFFFFF"
	if isDark {
		foreground = "#FFFFFF"
		muted = "#FFFFFF"
		border = "#FFFFFF"
		grid = "#FFFFFF4D"
		background = "#101012"
	}
	beginAtZero := false

	return shared.AdvancedChartConfig{
		Type: "line",
		Data: shared.AdvancedChartData{
			Labels: labels,
			Datasets: []shared.AdvancedChartDataset{
				{
					Label:                label,
					Data:                 data,
					BorderColor:          color,
					BackgroundColor:      color,
					PointBackgroundColor: color,
					PointBorderColor:     color,
					BorderWidth:          2,
					PointRadius:          1,
					PointHoverRadius:     6,
					Fill:                 false,
					Tension:              0.2,
				},
			},
		},
		Options: shared.AdvancedChartOptions{
			Responsive:          true,
			MaintainAspectRatio: false,
			Animation:           false,
			Interaction: &shared.Interaction{
				Intersect: false,
				Axis:      "xy",
				Mode:      "index",
			},
			Plugins: &shared.Plugins{
				Legend: &shared.PluginLegend{
					Display: true,
					Labels: &shared.PluginLegendLabels{
						Color: foreground,
					},
				},
				Tooltip: &shared.PluginTooltip{
					BackgroundColor: background,
					BodyColor:       muted,
					TitleColor:      foreground,
					BorderColor:     border,
					BorderWidth:     1,
				},
			},
			Scales: map[string]shared.AxisScale{
				"x": {
					Type:         "time",
					Distribution: "linear",
					Ticks: &shared.AxisTicks{
						Color:         muted,
						MaxTicksLimit: 8,
					},
					Grid: &shared.AxisGrid{
						Display: new(false),
					},
				},
				"y": {
					Offset:      new(true),
					BeginAtZero: &beginAtZero,
					Ticks: &shared.AxisTicks{
						Color: muted,
					},
					Grid: &shared.AxisGrid{
						Display: new(true),
						Color:   grid,
					},
					Border: &shared.AxisBorder{
						Display: true,
						Color:   border,
					},
				},
			},
		},
	}
}
