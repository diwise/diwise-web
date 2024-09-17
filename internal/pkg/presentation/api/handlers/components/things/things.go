package things

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/locale"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
)

func NewThingsPage(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")

		ctx = helpers.Decorate(
			r.Context(),
			components.CurrentComponent, "things",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
		offset, limit := helpers.GetOffsetAndLimit(r)
		mapview := false

		args := r.URL.Query()
		if mv, ok := args["mapview"]; ok && mv[0] == "true" {
			mapview = true
			offset = 0
			limit = 1000
		}

		helpers.SanitizeParams(args, "page", "limit", "offset")

		tags := make(chan []string)
		types := make(chan []string)

		go do(func() []string {
			t, err := app.GetTags(ctx)
			if err != nil {
				return []string{}
			}
			return t
		}, tags)

		go do(func() []string {
			t, err := app.GetTypes(ctx)
			if err != nil {
				return []string{}
			}
			return t
		}, types)

		result, err := app.GetThings(ctx, offset, limit, args)
		if err != nil {
			http.Error(w, "could not fetch things", http.StatusInternalServerError)
			return
		}

		pageIndex_, _ := strconv.Atoi(pageIndex)
		pageLast := int(math.Ceil(float64(result.TotalRecords) / float64(limit)))

		model := components.ThingsListViewModel{
			Things:  make([]components.ThingViewModel, 0),
			Pageing: getPaging(pageIndex_, pageLast, limit, offset, helpers.PagerIndexes(pageIndex_, pageLast), args),
			Tags:    <-tags,
			Types:   <-types,
			MapView: mapview,
		}

		for _, thing := range result.Things {
			tvm := toViewModel(thing)
			model.Things = append(model.Things, tvm)
		}

		thingList := components.ThingsList(localizer, model)
		page := components.StartPage(version, localizer, assets, thingList)

		ctx = helpers.Decorate(
			ctx,
			components.PageIndex, pageIndex_,
			components.PageLast, pageLast,
			components.PageSize, limit,
		)

		err = page.Render(ctx, w)
		if err != nil {
			http.Error(w, fmt.Sprintf("could not render things page - %s", err.Error()), http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusOK)

	}
	return http.HandlerFunc(fn)
}

func do[T any](fn func() T, ch chan T) {
	ch <- fn()
}

func NewThingsDataList(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")

		ctx = helpers.Decorate(
			r.Context(),
			components.CurrentComponent, "things",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
		offset, limit := helpers.GetOffsetAndLimit(r)
		mapview := false

		args := r.URL.Query()
		if mv, ok := args["mapview"]; ok && mv[0] == "true" {
			mapview = true
			offset = 0
			limit = 1000
		}

		helpers.SanitizeParams(args, "page", "limit", "offset")

		result, err := app.GetThings(ctx, offset, limit, args)
		if err != nil {
			http.Error(w, "could not fetch sensors", http.StatusInternalServerError)
			return
		}

		pageIndex_, _ := strconv.Atoi(pageIndex)
		pageLast := int(math.Ceil(float64(result.TotalRecords) / float64(limit)))

		model := components.ThingsListViewModel{
			Things:  make([]components.ThingViewModel, 0),
			Pageing: getPaging(pageIndex_, pageLast, limit, offset, helpers.PagerIndexes(pageIndex_, pageLast), args),
			MapView: mapview,
		}

		for _, thing := range result.Things {
			tvm := toViewModel(thing)
			model.Things = append(model.Things, tvm)
		}

		var tblComp, mapComp templ.Component
		if model.MapView {
			mapComp = components.ThingsMap(localizer, model)
			tblComp = templ.NopComponent
		} else {
			mapComp = templ.NopComponent
			tblComp = components.ThingsTable(localizer, model)
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

func NewThingsTable(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")

		ctx = helpers.Decorate(
			r.Context(),
			components.CurrentComponent, "things",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
		offset, limit := helpers.GetOffsetAndLimit(r)

		args := r.URL.Query()
		helpers.SanitizeParams(args, "page", "limit", "offset")

		result, err := app.GetThings(ctx, offset, limit, args)
		if err != nil {
			http.Error(w, "could not fetch things", http.StatusInternalServerError)
			return
		}

		pageIndex_, _ := strconv.Atoi(pageIndex)
		pageLast := int(math.Ceil(float64(result.TotalRecords) / float64(limit)))

		model := components.ThingsListViewModel{
			Things:  make([]components.ThingViewModel, 0),
			Pageing: getPaging(pageIndex_, pageLast, limit, offset, helpers.PagerIndexes(pageIndex_, pageLast), args),
			MapView: false,
		}

		for _, thing := range result.Things {
			tvm := toViewModel(thing)
			model.Things = append(model.Things, tvm)
		}

		component := components.ThingsTable(localizer, model)

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

func toViewModel(thing application.Thing) components.ThingViewModel {
	tvm := components.ThingViewModel{
		ThingID:      thing.ThingID,
		ID:           thing.ID,
		Type:         thing.Type,
		Latitude:     thing.Location.Latitude,
		Longitude:    thing.Location.Longitude,
		Tenant:       thing.Tenant,
		Tags:         thing.Tags,
		Measurements: make([]components.MeasurementViewModel, 0),
	}

	for _, m := range thing.Measurements {
		mvm := components.MeasurementViewModel{
			ID:          m.ID,
			Timestamp:   m.Timestamp,
			Urn:         m.Urn,
			BoolValue:   m.BoolValue,
			StringValue: m.StringValue,
			Unit:        m.Unit,
			Value:       m.Value,
		}
		tvm.Measurements = append(tvm.Measurements, mvm)
	}

	return tvm
}

func getPaging(pageIndex, pageLast, pageSize, offset int, pages []int64, args url.Values) components.PagingViewModel {
	return components.PagingViewModel{
		PageIndex: pageIndex,
		PageLast:  pageLast,
		PageSize:  pageSize,
		Offset:    offset,
		Pages:     pages,
		Query:     args.Encode(),
		TargetURL: "/components/tables/things",
		TargetID:  "#tableview",
	}
}
