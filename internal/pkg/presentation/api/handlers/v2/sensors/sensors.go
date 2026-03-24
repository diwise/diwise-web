package sensors

import (
	"context"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	featuresensors "github.com/diwise/diwise-web/internal/pkg/presentation/webv2/components/features/sensors"
	v2layout "github.com/diwise/diwise-web/internal/pkg/presentation/webv2/components/layout"

	. "github.com/diwise/frontend-toolkit"
)

func NewSensorsPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "sensors",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		model, err := composeListModel(ctx, r, app, true)
		if err != nil {
			http.Error(w, "could not fetch sensors", http.StatusInternalServerError)
			return
		}

		content := featuresensors.SensorsPage(localizer, model)
		page := templ.Component(v2layout.StartPage(version, localizer, assets, content))
		if helpers.IsHxRequest(r) {
			page = v2layout.AppShell(localizer, assets, content)
		}
		helpers.WriteComponentResponse(ctx, w, r, page, 32*1024, 0)
	}
}

func NewSensorsTable(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "sensors",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		model, err := composeListModel(ctx, r, app, false)
		if err != nil {
			http.Error(w, "could not fetch sensors", http.StatusInternalServerError)
			return
		}

		component := featuresensors.SensorsTableSection(localizer, model)
		helpers.WriteComponentResponse(ctx, w, r, component, 16*1024, 0)
	}
}

func NewSensorsDataList(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "sensors",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		model, err := composeListModel(ctx, r, app, false)
		if err != nil {
			http.Error(w, "could not fetch sensors", http.StatusInternalServerError)
			return
		}

		component := featuresensors.SensorsDataList(localizer, model)
		helpers.WriteComponentResponse(ctx, w, r, component, 16*1024, 0)
	}
}

func composeListModel(ctx context.Context, r *http.Request, app application.DeviceManagement, includePageMeta bool) (featuresensors.SensorsPageViewModel, error) {
	pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
	offset, limit := helpers.GetOffsetAndLimit(r)
	showMap := r.URL.Query().Get("mapview") == "true"

	args := r.URL.Query()
	helpers.SanitizeParams(args, "page", "limit", "offset")
	selectedTypes := normalizeTypeFilter(args)

	if showMap {
		offset = 0
		limit = 1000
	}

	result, err := app.GetDevices(ctx, offset, limit, args)
	if err != nil {
		return featuresensors.SensorsPageViewModel{}, err
	}

	pageIndexInt, _ := strconv.Atoi(pageIndex)
	pageLast := int(math.Ceil(float64(result.TotalRecords) / float64(limit)))

	model := featuresensors.SensorsPageViewModel{
		MapView: showMap,
		Sensors: make([]featuresensors.SensorViewModel, 0, len(result.Devices)),
		Filters: featuresensors.FiltersViewModel{
			Search:        r.URL.Query().Get("search"),
			LastSeen:      r.URL.Query().Get("lastseen"),
			LastSeenDate:  parseLastSeen(r.URL.Query().Get("lastseen")),
			LocaleTag:     localeTagFromHeader(r.Header.Get("Accept-Language")),
			SelectedTypes: selectedTypes,
			Active:        r.URL.Query().Get("active"),
			Online:        r.URL.Query().Get("online"),
			PageSize:      limit,
		},
		Paging: featuresensors.PagingViewModel{
			PageIndex:  max(pageIndexInt, 1),
			PageLast:   max(pageLast, 1),
			PageSize:   limit,
			TotalCount: result.TotalRecords,
			Query:      args.Encode(),
			TargetURL:  "/v2/components/sensors/list",
			TargetID:   "#tableOrMap",
		},
	}

	for _, device := range result.Devices {
		model.Sensors = append(model.Sensors, toViewModel(device))
	}

	if includePageMeta {
		stats, err := getStatistics(ctx, app)
		if err != nil {
			return featuresensors.SensorsPageViewModel{}, err
		}

		model.Statistics = stats
		model.DeviceProfiles = getDeviceProfiles(ctx, app)
	}

	return model, nil
}

func normalizeTypeFilter(args url.Values) []string {
	rawTypes := args["type"]
	if len(rawTypes) == 0 {
		return nil
	}

	selectedTypes := make([]string, 0, len(rawTypes))
	for _, rawValue := range rawTypes {
		for _, part := range strings.Split(rawValue, ",") {
			part = strings.TrimSpace(part)
			if part == "" || slices.Contains(selectedTypes, part) {
				continue
			}
			selectedTypes = append(selectedTypes, part)
		}
	}

	args.Del("type")
	for _, value := range selectedTypes {
		args.Add("type", value)
	}

	return selectedTypes
}

func getStatistics(ctx context.Context, app application.DeviceManagement) (featuresensors.StatisticsViewModel, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	stats, err := app.GetStatistics(ctx)
	if err != nil {
		return featuresensors.StatisticsViewModel{}, err
	}

	return featuresensors.StatisticsViewModel{
		Total:    stats.Total,
		Active:   stats.Active,
		Inactive: stats.Inactive,
		Online:   stats.Online,
		Unknown:  stats.Unknown,
	}, nil
}

func getDeviceProfiles(ctx context.Context, app application.DeviceManagement) []string {
	profiles := app.GetDeviceProfiles(ctx)
	names := make([]string, 0, len(profiles)+1)
	for _, p := range profiles {
		names = append(names, p.Decoder)
	}

	if !slices.Contains(names, "unknown") {
		names = append(names, "unknown")
	}

	slices.Sort(names)
	return names
}

func toViewModel(device application.Device) featuresensors.SensorViewModel {
	lastSeen := time.Time{}

	if device.SensorStatus != nil {
		lastSeen = device.SensorStatus.ObservedAt
	}
	if device.DeviceState != nil {
		lastSeen = device.DeviceState.ObservedAt
	}

	viewModel := featuresensors.SensorViewModel{
		Active:       device.Active,
		DeviceID:     device.DeviceID,
		DevEUI:       device.SensorID,
		Name:         device.Name,
		BatteryLevel: batteryLevel(device),
		LastSeen:     lastSeen,
		HasAlerts:    len(device.Alarms) > 0,
		Latitude:     device.Location.Latitude,
		Longitude:    device.Location.Longitude,
	}

	if device.SensorProfile != nil {
		viewModel.Type = device.SensorProfile.Name
	}
	if device.DeviceState != nil {
		viewModel.Online = device.DeviceState.Online
	}

	return viewModel
}

func batteryLevel(device application.Device) int {
	if device.SensorStatus != nil && device.SensorStatus.BatteryLevel != 0 {
		return device.SensorStatus.BatteryLevel
	}
	return -1
}

func parseLastSeen(value string) time.Time {
	if value == "" {
		return time.Time{}
	}

	for _, layout := range []string{time.DateOnly, "2006-01-02T15:04", time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}

	return time.Time{}
}

func localeTagFromHeader(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "en-US"
	}

	tag, _, _ := strings.Cut(value, ",")
	tag, _, _ = strings.Cut(tag, ";")
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "en-US"
	}

	return tag
}
