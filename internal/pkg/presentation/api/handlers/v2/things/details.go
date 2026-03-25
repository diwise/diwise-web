package things

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/application/client"
	appthings "github.com/diwise/diwise-web/internal/application/things"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	featuresthings "github.com/diwise/diwise-web/internal/pkg/presentation/webv2/components/features/things"
	v2layout "github.com/diwise/diwise-web/internal/pkg/presentation/webv2/components/layout"
	shared "github.com/diwise/diwise-web/internal/pkg/presentation/webv2/components/shared"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"

	. "github.com/diwise/frontend-toolkit"
)

func NewThingDetailsPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app thingsApp) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "things",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		editMode := r.URL.Query().Get("mode") == "edit"
		model, err := composeDetailsModel(ctx, id, app, editMode)
		if err != nil {
			http.Error(w, "could not fetch thing", http.StatusInternalServerError)
			return
		}

		content := featuresthings.ThingDetailsPage(localizer, model)
		if editMode {
			content = featuresthings.EditThingDetailsPage(localizer, model)
		}

		page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
		if helpers.IsHxRequest(r) {
			page = v2layout.AppShell(localizer, assets, content)
		}

		helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, 0)
	}
}

func NewSaveThingDetailsPage(_ context.Context, _ LocaleBundle, _ AssetLoaderFunc, app thingsApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "could not parse form data", http.StatusBadRequest)
			return
		}

		if err := app.UpdateThing(r.Context(), id, buildThingUpdateFields(r.Form)); err != nil {
			http.Error(w, "could not update thing", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/v2/things/%s", id), http.StatusFound)
	}
}

func NewDeleteThingDetailsPage(_ context.Context, _ LocaleBundle, _ AssetLoaderFunc, app thingsApp) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		if err := app.DeleteThing(r.Context(), id); err != nil {
			http.Error(w, "could not delete thing", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/v2/things", http.StatusFound)
	}
}

func NewThingMeasurementComponentHandler(ctx context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app thingsApp) http.HandlerFunc {
	log := logging.GetFromContext(ctx)

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := logging.NewContextWithLogger(r.Context(), log)
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		activeMeasurement := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("measurement")))
		if activeMeasurement == "" {
			component := featuresthings.ThingMeasurementPanel(l10n.For(r.Header.Get("Accept-Language")), featuresthings.ThingMeasurementPanelProps{
				Empty: true,
			})
			helpers.WriteComponentResponse(ctx, w, r, component, 8*1024, 0)
			return
		}

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		startTime := getThingTime(r, "timeAt", startOfDay(time.Now().UTC()))
		endTime := getThingTime(r, "endTimeAt", endOfDay(time.Now().UTC()))
		query := measurementQuery(activeMeasurement, startTime, endTime)

		thing, err := app.GetThing(ctx, id, query)
		if err != nil {
			http.Error(w, "could not fetch thing measurements", http.StatusInternalServerError)
			return
		}

		panel := featuresthings.ThingMeasurementPanel(localizer, featuresthings.ThingMeasurementPanelProps{
			Chart: featuresthings.ThingMeasurementChartComponent(thingMeasurementChartConfig(r, localizer, activeMeasurement, thing)),
			Rows:  measurementRows(thing.Values),
			Empty: len(thing.Values) == 0 || countMeasurements(thing.Values) == 0,
		})
		helpers.WriteComponentResponse(ctx, w, r, panel, 24*1024, 5*time.Minute)
	}
}

func composeDetailsModel(ctx context.Context, id string, app thingsApp, includeEditOptions bool) (featuresthings.ThingDetailsPageViewModel, error) {
	thing, err := app.GetThing(ctx, id, nil)
	if err != nil {
		return featuresthings.ThingDetailsPageViewModel{}, err
	}

	latestValues, err := app.GetLatestValues(ctx, id)
	if err != nil {
		return featuresthings.ThingDetailsPageViewModel{}, err
	}

	model := featuresthings.ThingDetailsPageViewModel{
		Thing:              toViewModel(thing),
		LatestValues:       make([]featuresthings.LatestMeasurementViewModel, 0, len(latestValues)),
		ConnectedNames:     make([]string, 0, len(thing.RefDevices)),
		ValidSensors:       make([]featuresthings.SensorOption, 0),
		MeasurementOptions: make([]featuresthings.MeasurementOption, 0, len(latestValues)),
	}

	for _, ref := range thing.RefDevices {
		if ref.DeviceID == "" || slices.Contains(model.ConnectedNames, ref.DeviceID) {
			continue
		}
		model.ConnectedNames = append(model.ConnectedNames, ref.DeviceID)
	}

	for _, measurement := range latestValues {
		label := latestMeasurementLabel(id, measurement)
		item := featuresthings.LatestMeasurementViewModel{
			ID:        measurement.ID,
			Label:     label,
			Timestamp: measurement.Timestamp,
			Unit:      measurement.Unit,
			Value:     measurement.Value,
			BoolValue: measurement.BoolValue,
		}
		if measurement.StringValue != nil {
			item.StringValue = *measurement.StringValue
		}
		model.LatestValues = append(model.LatestValues, item)
		if label != "" && !slices.ContainsFunc(model.MeasurementOptions, func(option featuresthings.MeasurementOption) bool {
			return option.Value == label
		}) {
			model.MeasurementOptions = append(model.MeasurementOptions, featuresthings.MeasurementOption{
				Value: label,
				Label: label,
			})
		}
	}

	slices.SortFunc(model.LatestValues, func(a, b featuresthings.LatestMeasurementViewModel) int {
		return cmp.Compare(a.Label, b.Label)
	})
	slices.SortFunc(model.MeasurementOptions, func(a, b featuresthings.MeasurementOption) int {
		return cmp.Compare(a.Label, b.Label)
	})
	if len(model.MeasurementOptions) > 0 {
		model.SelectedMeasurement = model.MeasurementOptions[0].Value
	}

	if includeEditOptions {
		model.Organisations = app.GetTenants(ctx)
		model.TagOptions, _ = app.GetTags(ctx)
		validSensors, _ := app.GetValidSensors(ctx, thing.ValidURNs)
		for _, sensor := range validSensors {
			model.ValidSensors = append(model.ValidSensors, featuresthings.SensorOption{
				Value: sensor.DeviceID,
				Label: fmt.Sprintf("%s (%s)", sensor.SensorID, sensor.Decoder),
			})
		}
		slices.SortFunc(model.ValidSensors, func(a, b featuresthings.SensorOption) int {
			return cmp.Compare(a.Label, b.Label)
		})
	}

	return model, nil
}

func latestMeasurementLabel(thingID string, measurement appthings.Measurement) string {
	trimmed := strings.TrimSpace(measurement.ID)
	if trimmed == "" {
		return measurement.Urn
	}

	trimmed = strings.TrimPrefix(trimmed, thingID)
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		return measurement.Urn
	}

	return strings.ReplaceAll(trimmed, "/", "-")
}

func buildThingUpdateFields(form url.Values) map[string]any {
	asFloat := func(s string) (float64, bool) {
		if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
			return f, true
		}
		return 0, false
	}

	fields := map[string]any{}
	if tags := normalizeListValues(form["tags"]); len(tags) > 0 {
		fields["tags"] = tags
	}
	if refs := normalizeListValues(form["currentDevice"]); len(refs) > 0 {
		devices := make([]appthings.RefDevice, 0, len(refs))
		for _, ref := range refs {
			devices = append(devices, appthings.RefDevice{DeviceID: ref})
		}
		fields["refDevices"] = devices
	}

	for formKey, fieldKey := range map[string]string{
		"name":            "name",
		"alternativeName": "alternativeName",
		"description":     "description",
	} {
		if value := strings.TrimSpace(form.Get(formKey)); value != "" {
			fields[fieldKey] = value
		}
	}

	if organisation := strings.TrimSpace(form.Get("organisation")); organisation != "" {
		fields["tenant"] = organisation
	}

	for _, key := range []string{"latitude", "longitude"} {
		if value := strings.TrimSpace(form.Get(key)); value != "" {
			if _, ok := fields["location"]; !ok {
				fields["location"] = client.Location{}
			}
			if parsed, ok := asFloat(value); ok {
				location := fields["location"].(client.Location)
				if key == "latitude" {
					location.Latitude = parsed
				} else {
					location.Longitude = parsed
				}
				fields["location"] = location
			}
		}
	}

	for _, key := range []string{"maxl", "maxd", "angle", "offset"} {
		if value := strings.TrimSpace(form.Get(key)); value != "" {
			if parsed, ok := asFloat(value); ok {
				fields[key] = parsed
			}
		}
	}

	return fields
}

func normalizeListValues(values []string) []string {
	items := make([]string, 0)
	for _, value := range values {
		for _, part := range normalizeCSVList(value) {
			if slices.Contains(items, part) {
				continue
			}
			items = append(items, part)
		}
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func normalizeCSVList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	items := make([]string, 0)
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" || slices.Contains(items, part) {
			continue
		}
		items = append(items, part)
	}

	return items
}

func getThingTime(r *http.Request, key string, def time.Time) time.Time {
	layout := "2006-01-02T15:04"
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return def
	}

	parsed, err := time.Parse(layout, value)
	if err != nil {
		return def
	}
	return parsed
}

func startOfDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func endOfDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 23, 59, 0, 0, value.Location())
}

func measurementQuery(measurement string, startTime, endTime time.Time) url.Values {
	n := strings.ReplaceAll(measurement, "-", "/")
	query := url.Values{}
	query.Add("timerel", "between")
	query.Add("timeat", startTime.Format(time.RFC3339))
	query.Add("endTimeAt", endTime.Format(time.RFC3339))
	query.Add("options", "groupByRef")
	query.Add("n", n)

	if strings.HasPrefix(n, "10351/") || strings.HasPrefix(n, "3302/") {
		query.Add("timeunit", "hour")
		query.Add("vb", "true")
		query.Del("options")
	}

	if strings.HasPrefix(n, "3350/") {
		query.Del("options")
		if n == "3350/5544" {
			query.Add("op", "gt")
			query.Add("value", "0")
		}
		if n == "3350/5850" {
			query.Add("distinct", "vb")
		}
	}

	return query
}

func thingMeasurementChartConfig(r *http.Request, l10n Localizer, measurement string, thing appthings.Thing) shared.AdvancedChartConfig {
	isDark := helpers.IsDarkMode(r)
	theme := chartTheme(isDark)
	datasets := make([]shared.AdvancedChartDataset, 0, len(thing.Values))
	labels := thingMeasurementLabels(thing.Values)
	for index, group := range thing.Values {
		datasets = append(datasets, thingMeasurementDataset(l10n, measurement, group, index, isDark))
	}
	if len(datasets) == 0 {
		datasets = append(datasets, shared.AdvancedChartDataset{
			Label: measurement,
			Data:  []any{},
		})
	}

	beginAtZero := false
	yScale := shared.AxisScale{
		Offset:      boolPtr(true),
		BeginAtZero: &beginAtZero,
		Ticks: &shared.AxisTicks{
			Color: theme.MutedForeground,
		},
		Grid: &shared.AxisGrid{
			Display: boolPtr(true),
			Color:   theme.Grid,
		},
		Border: &shared.AxisBorder{
			Display: true,
			Color:   theme.Border,
		},
	}
	if maxDistanceChartMeasurement(measurement) && thing.TypeValues.MaxDistance != nil && *thing.TypeValues.MaxDistance > 0 {
		maxValue := math.Ceil(*thing.TypeValues.MaxDistance)
		minValue := 0.0
		yScale.Min = &minValue
		yScale.Max = &maxValue
	}

	return shared.AdvancedChartConfig{
		Type: thingChartType(measurement),
		Data: shared.AdvancedChartData{
			Labels:   labels,
			Datasets: datasets,
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
						Color: theme.Foreground,
					},
				},
				Tooltip: &shared.PluginTooltip{
					BackgroundColor: theme.Background,
					BodyColor:       theme.MutedForeground,
					TitleColor:      theme.Foreground,
					BorderColor:     theme.Border,
					BorderWidth:     1,
				},
			},
			Scales: map[string]shared.AxisScale{
				"x": measurementTimeScale(theme),
				"y": yScale,
			},
		},
	}
}

func maxDistanceChartMeasurement(measurement string) bool {
	urn := strings.ReplaceAll(measurement, "-", "/")
	return strings.HasSuffix(urn, "/3")
}

func thingMeasurementDataset(l10n Localizer, measurement string, values []appthings.Measurement, index int, isDark bool) shared.AdvancedChartDataset {
	color := thingChartColor(index, isDark)
	dataset := shared.AdvancedChartDataset{
		Label:                localizedMeasurementSeriesLabel(l10n, measurement, values, index),
		Data:                 make([]any, 0, len(values)),
		BorderColor:          color,
		BackgroundColor:      color,
		PointBackgroundColor: color,
		PointBorderColor:     color,
		BorderWidth:          2,
		PointRadius:          1,
		PointHoverRadius:     6,
		Fill:                 false,
		Tension:              0.2,
		Stepped:              thingChartType(measurement) != "line",
	}

	previousBool := 0
	for _, value := range values {
		switch {
		case value.Value != nil:
			dataset.Data = append(dataset.Data, *value.Value)
		case value.Count != nil:
			dataset.Data = append(dataset.Data, *value.Count)
		case value.BoolValue != nil:
			current := 0
			if *value.BoolValue {
				current = 1
			}
			if current != previousBool {
				dataset.Data = append(dataset.Data, previousBool)
				previousBool = current
			}
			dataset.Data = append(dataset.Data, current)
		default:
			dataset.Data = append(dataset.Data, nil)
		}
	}

	return dataset
}

func localizedMeasurementSeriesLabel(l10n Localizer, measurement string, values []appthings.Measurement, index int) string {
	if measurement != "" {
		return featuresthings.LocalizedMeasurementLabel(l10n, measurement)
	}
	return fmt.Sprintf("Series %d", index+1)
}

func thingMeasurementLabels(groups [][]appthings.Measurement) []string {
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		labels := make([]string, 0, len(group))
		for _, value := range group {
			labels = append(labels, value.Timestamp.Format("2006-01-02 15:04"))
		}
		return labels
	}
	return []string{}
}

func thingChartColor(index int, isDark bool) string {
	colors := []string{"#1F1F25", "#C24E18", "#1D4ED8", "#059669", "#7C3AED"}
	if isDark {
		colors = []string{"#FFFFFF", "#C24E18", "#93C5FD", "#6EE7B7", "#C4B5FD"}
	}
	return colors[index%len(colors)]
}

func thingChartType(measurement string) string {
	urn := strings.ReplaceAll(measurement, "-", "/")
	if strings.HasPrefix(urn, "10351/") || urn == "3350-5544" || urn == "3350/5544" {
		return "bar"
	}
	return "line"
}

func countMeasurements(groups [][]appthings.Measurement) int {
	total := 0
	for _, group := range groups {
		total += len(group)
	}
	return total
}

func measurementRows(groups [][]appthings.Measurement) []featuresthings.MeasurementTableRow {
	rows := make([]featuresthings.MeasurementTableRow, 0)
	for _, group := range groups {
		for _, value := range group {
			rows = append(rows, featuresthings.MeasurementTableRow{
				Timestamp: value.Timestamp.Format("2006-01-02 15:04"),
				Value:     measurementRowValue(value),
			})
		}
	}
	slices.SortFunc(rows, func(a, b featuresthings.MeasurementTableRow) int {
		return cmp.Compare(b.Timestamp, a.Timestamp)
	})
	return rows
}

func measurementRowValue(value appthings.Measurement) string {
	switch {
	case value.Value != nil:
		return fmt.Sprintf("%.1f", *value.Value)
	case value.Count != nil:
		return fmt.Sprintf("%.0f", *value.Count)
	case value.BoolValue != nil:
		if *value.BoolValue {
			return "true"
		}
		return "false"
	case value.StringValue != nil:
		return *value.StringValue
	default:
		return "-"
	}
}

type chartThemeConfig struct {
	Foreground      string
	Background      string
	MutedForeground string
	Border          string
	Grid            string
}

func chartTheme(isDark bool) chartThemeConfig {
	if isDark {
		return chartThemeConfig{
			Foreground:      "#FFFFFF",
			Background:      "#101012",
			MutedForeground: "#FFFFFF",
			Border:          "#FFFFFF",
			Grid:            "#FFFFFF4D",
		}
	}

	return chartThemeConfig{
		Foreground:      "#1F1F25",
		Background:      "#FFFFFF",
		MutedForeground: "#444450",
		Border:          "#1F1F25",
		Grid:            "#E2E2E8",
	}
}

func measurementTimeScale(theme chartThemeConfig) shared.AxisScale {
	return shared.AxisScale{
		Type:         "time",
		Distribution: "linear",
		Ticks: &shared.AxisTicks{
			Color:         theme.MutedForeground,
			MaxTicksLimit: 8,
		},
		Grid: &shared.AxisGrid{
			Display: boolPtr(false),
			Color:   theme.Border,
		},
		Border: &shared.AxisBorder{
			Display: true,
			Color:   theme.Border,
		},
		Time: &shared.ScaleTime{
			Unit:          "hour",
			TooltipFormat: "yyyy-MM-dd HH:mm",
			Parser:        "yyyy-MM-dd HH:mm",
			DisplayFormats: map[string]string{
				"hour": "HH:mm",
				"day":  "yyyy-MM-dd",
			},
		},
	}
}

func boolPtr(v bool) *bool {
	return &v
}
