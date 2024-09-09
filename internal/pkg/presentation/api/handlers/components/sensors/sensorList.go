package sensors

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/locale"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
)

func NewSensorsPage(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")

		ctx = helpers.Decorate(
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
			Sensors: make([]components.SensorViewModel, 0),
			Pageing: getPaging(pageIndex_, pageLast, limit, offset, helpers.PagerIndexes(pageIndex_, pageLast), args),
		}

		for _, sensor := range result.Sensors {
			tvm := toViewModel(sensor)
			tvm.BatteryLevel = getBatterLevel(ctx, app, sensor.DeviceID)
			model.Sensors = append(model.Sensors, tvm)
		}

		model.Statistics = getStatistics(ctx, app)

		if mv, ok := args["mapview"]; ok && mv[0] == "true" {
			model.MapView = true
		}

		sensorList := components.SensorsList(localizer, model)
		page := components.StartPage(version, localizer, assets, sensorList)

		ctx = helpers.Decorate(
			ctx,
			components.PageIndex, pageIndex_,
			components.PageLast, pageLast,
			components.PageSize, limit,
		)

		w.WriteHeader(http.StatusOK)

		err = page.Render(ctx, w)
		if err != nil {
			http.Error(w, fmt.Sprintf("could not render things page - %s", err.Error()), http.StatusInternalServerError)
		}

	}
	return http.HandlerFunc(fn)
}

func NewSensorsTable(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")

		ctx = helpers.Decorate(
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
			Pageing:    getPaging(pageIndex_, pageLast, limit, offset, helpers.PagerIndexes(pageIndex_, pageLast), args),
		}

		for _, sensor := range result.Sensors {
			tvm := toViewModel(sensor)
			tvm.BatteryLevel = getBatterLevel(ctx, app, sensor.DeviceID)
			model.Sensors = append(model.Sensors, tvm)
		}

		component := components.SensorsTable(localizer, model)

		ctx = helpers.Decorate(
			ctx,
			components.PageIndex, pageIndex_,
			components.PageLast, pageLast,
			components.PageSize, limit,
		)

		err = component.Render(ctx, w)
		if err != nil {
			http.Error(w, fmt.Sprintf("could not render things page - %s", err.Error()), http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusOK)

	}
	return http.HandlerFunc(fn)
}

func NewSensorsDataList(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")

		ctx = helpers.Decorate(
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
			Pageing:    getPaging(pageIndex_, pageLast, limit, offset, helpers.PagerIndexes(pageIndex_, pageLast), args),
		}

		for _, sensor := range result.Sensors {
			tvm := toViewModel(sensor)
			tvm.BatteryLevel = getBatterLevel(ctx, app, sensor.DeviceID)
			model.Sensors = append(model.Sensors, tvm)
		}

		if mv, ok := args["mapview"]; ok && mv[0] == "true" {
			model.MapView = true
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

		err = component.Render(ctx, w)
		if err != nil {
			http.Error(w, fmt.Sprintf("could not render things page - %s", err.Error()), http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusOK)

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

func getBatterLevel(ctx context.Context, app application.DeviceManagement, deviceID string) int {
	batteryLevelID := fmt.Sprintf("%s/3/9", deviceID)
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
		HasAlerts:    false,
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

	return s
}

func getPaging(pageIndex, pageLast, pageSize, offset int, pages []int64, args url.Values) components.PagingViewModel {
	return components.PagingViewModel{
		PageIndex: pageIndex,
		PageLast:  pageLast,
		PageSize:  pageSize,
		Offset:    offset,
		Pages:     pages,
		Query:     args.Encode(),
		TargetURL: "/components/tables/sensors",
		TargetID:  "#tableview",
	}
}
