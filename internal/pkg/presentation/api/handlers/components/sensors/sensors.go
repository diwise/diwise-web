package sensors

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"

	. "github.com/diwise/frontend-toolkit"
)

func NewSensorsPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {

		ctx = helpers.Decorate(
			r.Context(),
			components.CurrentComponent, "sensors",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
		offset, limit := helpers.GetOffsetAndLimit(r)
		showMap := false

		args := r.URL.Query()
		helpers.SanitizeParams(args, "page", "limit", "offset")

		if mv, ok := args["mapview"]; ok && mv[0] == "true" {
			showMap = true
		}

		if showMap {
			offset = 0
			limit = 1000
		}

		result, err := app.GetSensors(ctx, offset, limit, args)
		if err != nil {
			http.Error(w, "could not fetch sensors", http.StatusInternalServerError)
			return
		}

		pageIndex_, _ := strconv.Atoi(pageIndex)
		pageLast := int(math.Ceil(float64(result.TotalRecords) / float64(limit)))

		model := components.SensorListViewModel{
			Sensors:        make([]components.SensorViewModel, 0),
			Pageing:        getPaging(pageIndex_, pageLast, limit, result.Count, result.TotalRecords, offset, helpers.PagerIndexes(pageIndex_, pageLast), args),
			MapView:        showMap,
			DeviceProfiles: make([]string, 0),
		}

		for _, sensor := range result.Sensors {
			tvm := toViewModel(sensor)
			tvm.BatteryLevel = getBatteryLevel(ctx, app, sensor)
			model.Sensors = append(model.Sensors, tvm)
		}

		profiles := app.GetDeviceProfiles(ctx)
		for _, p := range profiles {
			model.DeviceProfiles = append(model.DeviceProfiles, p.Decoder)
		}
		if !slices.Contains(model.DeviceProfiles, "unknown") {
			model.DeviceProfiles = append(model.DeviceProfiles, "unknown")
		}

		model.Statistics = getStatistics(ctx, app)

		sensorList := components.SensorsList(localizer, model)
		page := components.StartPage(version, localizer, assets, sensorList)

		ctx = helpers.Decorate(
			ctx,
			components.PageIndex, pageIndex_,
			components.PageLast, pageLast,
			components.PageSize, limit,
		)

		helpers.WriteComponentResponse(ctx, w, r, page, 10*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewSensorsTable(_ context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {

		ctx := helpers.Decorate(
			r.Context(),
			components.CurrentComponent, "sensors",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
		offset, limit := helpers.GetOffsetAndLimit(r)

		args := r.URL.Query()
		helpers.SanitizeParams(args, "page", "limit", "offset")

		result, err := app.GetSensors(ctx, offset, limit, args)
		if err != nil {
			http.Error(w, "could not fetch sensors", http.StatusInternalServerError)
			return
		}

		pageIndex_, _ := strconv.Atoi(pageIndex)
		pageLast := int(math.Ceil(float64(result.TotalRecords) / float64(limit)))

		model := components.SensorListViewModel{
			Statistics: components.StatisticsViewModel{},
			Sensors:    make([]components.SensorViewModel, 0),
			Pageing:    getPaging(pageIndex_, pageLast, limit, result.Count, result.TotalRecords, offset, helpers.PagerIndexes(pageIndex_, pageLast), args),
		}

		for _, sensor := range result.Sensors {
			tvm := toViewModel(sensor)
			tvm.BatteryLevel = getBatteryLevel(ctx, app, sensor)
			model.Sensors = append(model.Sensors, tvm)
		}

		component := components.SensorsTable(localizer, model)

		ctx = helpers.Decorate(
			ctx,
			components.PageIndex, pageIndex_,
			components.PageLast, pageLast,
			components.PageSize, limit,
		)

		helpers.WriteComponentResponse(ctx, w, r, component, 1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewSensorsDataList(_ context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {

		ctx := helpers.Decorate(
			r.Context(),
			components.CurrentComponent, "sensors",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
		offset, limit := helpers.GetOffsetAndLimit(r)
		showMap := false

		args := r.URL.Query()
		helpers.SanitizeParams(args, "page", "limit", "offset")

		if mv, ok := args["mapview"]; ok && mv[0] == "true" {
			showMap = true
		}

		if showMap {
			offset = 0
			limit = 1000
		}

		result, err := app.GetSensors(ctx, offset, limit, args)
		if err != nil {
			http.Error(w, "could not fetch sensors", http.StatusInternalServerError)
			return
		}

		pageIndex_, _ := strconv.Atoi(pageIndex)
		pageLast := int(math.Ceil(float64(result.TotalRecords) / float64(limit)))

		model := components.SensorListViewModel{
			Statistics: components.StatisticsViewModel{},
			Sensors:    make([]components.SensorViewModel, 0),
			Pageing:    getPaging(pageIndex_, pageLast, limit, result.Count, result.TotalRecords, offset, helpers.PagerIndexes(pageIndex_, pageLast), args),
			MapView:    showMap,
		}

		for _, sensor := range result.Sensors {
			tvm := toViewModel(sensor)
			tvm.BatteryLevel = getBatteryLevel(ctx, app, sensor)
			model.Sensors = append(model.Sensors, tvm)
		}

		var tblComp, mapComp templ.Component
		if model.MapView {
			mapComp = components.SensorMap(localizer, model)
			tblComp = templ.NopComponent
		} else {
			mapComp = templ.NopComponent
			tblComp = components.SensorsTable(localizer, model)
		}

		component := components.DataList(localizer, tblComp, mapComp, model.MapView)

		ctx = helpers.Decorate(
			ctx,
			components.PageIndex, pageIndex_,
			components.PageLast, pageLast,
			components.PageSize, limit,
		)

		helpers.WriteComponentResponse(ctx, w, r, component, 1024, 0)
	}

	return http.HandlerFunc(fn)
}

func getStatistics(ctx context.Context, app application.DeviceManagement) components.StatisticsViewModel {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	sumOfStuff := app.GetStatistics(ctx)

	stats := components.StatisticsViewModel{
		Total:    sumOfStuff.Total,
		Active:   sumOfStuff.Active,
		Inactive: sumOfStuff.Inactive,
		Online:   sumOfStuff.Online,
		Unknown:  sumOfStuff.Unknown,
	}

	return stats
}

func getBatteryLevel(ctx context.Context, app application.DeviceManagement, sensor application.Sensor) int {
	if sensor.DeviceStatus != nil {
		if sensor.DeviceStatus.BatteryLevel != 0 {
			return sensor.DeviceStatus.BatteryLevel
		}
	}

	batteryLevelID := fmt.Sprintf("%s/3/9", sensor.DeviceID)
	data, err := app.GetMeasurementData(ctx, batteryLevelID, application.WithLastN(true), application.WithLimit(1))
	if err != nil {
		return -1
	}

	if len(data.Values) > 0 {
		if len(data.Values) > 0 && data.Values[0].Value != nil {
			v := int(math.Min(math.Max(*data.Values[0].Value, 0), 100))
			return v
		}
	}

	return -1
}

func toViewModel(sensor application.Sensor) components.SensorViewModel {
	s := components.SensorViewModel{
		HasAlerts:    len(sensor.Alarms) > 0,
		Active:       sensor.Active,
		DeviceID:     sensor.DeviceID,
		DevEUI:       sensor.SensorID,
		Name:         sensor.Name,
		BatteryLevel: 0,
		LastSeen:     sensor.DeviceState.ObservedAt,
		Latitude:     sensor.Location.Latitude,
		Longitude:    sensor.Location.Longitude,
	}

	if sensor.DeviceProfile != nil {
		s.Type = sensor.DeviceProfile.Name
	}
	if sensor.DeviceState != nil {
		s.Online = sensor.DeviceState.Online
	}

	return s
}

func getPaging(pageIndex, pageLast, pageSize, count, total, offset int, pages []int64, args url.Values) components.PagingViewModel {
	return components.PagingViewModel{
		PageIndex:  pageIndex,
		PageLast:   pageLast,
		PageSize:   pageSize,
		Offset:     offset,
		Count:      count,
		TotalCount: total,
		Pages:      pages,
		Query:      args.Encode(),
		TargetURL:  "/components/tables/sensors",
		TargetID:   "#tableview",
	}
}
