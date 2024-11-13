package things

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/google/uuid"

	//lint:ignore ST1001 it is OK when we do it
	. "github.com/diwise/frontend-toolkit"
)

func NewThingsPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {

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

		helpers.SanitizeParams(args, "mapview", "page", "limit", "offset")

		tags, _ := app.GetTags(ctx)
		types, _ := app.GetTypes(ctx)

		result, err := app.GetThings(ctx, offset, limit, args)
		if err != nil {
			http.Error(w, "could not fetch things", http.StatusInternalServerError)
			return
		}

		pageIndex_, _ := strconv.Atoi(pageIndex)
		pageLast := int(math.Ceil(float64(result.TotalRecords) / float64(limit)))

		model := components.ThingsListViewModel{
			Things:  make([]components.ThingViewModel, 0),
			Pageing: getPaging(pageIndex_, pageLast, limit, result.Count, result.TotalRecords, offset, helpers.PagerIndexes(pageIndex_, pageLast), args),
			Tags:    tags,
			Types:   types,
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

		helpers.WriteComponentResponse(ctx, w, r, page, 1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewThingComponentHandler(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		ctx := helpers.Decorate(r.Context(),
			components.CurrentComponent, "things",
		)

		thingTypes, _ := app.GetTypes(ctx)

		newThingViewModel := components.NewThingViewModel{
			ThingType:     thingTypes,
			Organisations: app.GetTenants(ctx),
		}

		newThingViewModel.Tags, _ = app.GetTags(ctx)

		component := components.NewThing(localizer, assets, newThingViewModel)
		helpers.WriteComponentResponse(ctx, w, r, component, 1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewCreateThingComponentHandler(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
	log := logging.GetFromContext(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		ctx := logging.NewContextWithLogger(r.Context(), log)

		err := r.ParseForm()
		if err != nil {
			http.Error(w, "could not parse form data", http.StatusBadRequest)
			return
		}

		id := uuid.NewString()

		if !r.Form.Has("save") {
			http.Redirect(w, r, "/things", http.StatusTemporaryRedirect)
			return
		}

		thingType := r.Form.Get("type")
		thingSubType := ""

		thingName := r.Form.Get("name")
		thingOrg := r.Form.Get("organisation")
		thingDesc := r.Form.Get("description")

		if strings.Contains(thingType, ":") {
			parts := strings.Split(thingType, ":")
			thingType = parts[0]
			thingSubType = parts[1]
		}

		err = app.NewThing(ctx, application.Thing{
			ID:          id,
			Type:        thingType,
			SubType:     thingSubType,
			Name:        thingName,
			Description: thingDesc,
			Location: application.Location{
				Latitude:  0,
				Longitude: 0,
			},
			Tenant: thingOrg,
		})
		if err != nil {
			http.Error(w, "could not create new thing", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/things/%s?mode=edit", id), http.StatusMovedPermanently)
	}

	return http.HandlerFunc(fn)
}

func NewThingsDataList(_ context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {

		ctx := helpers.Decorate(
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

		helpers.SanitizeParams(args, "mapview", "page", "limit", "offset")

		result, err := app.GetThings(ctx, offset, limit, args)
		if err != nil {
			http.Error(w, "could not fetch sensors", http.StatusInternalServerError)
			return
		}

		pageIndex_, _ := strconv.Atoi(pageIndex)
		pageLast := int(math.Ceil(float64(result.TotalRecords) / float64(limit)))

		model := components.ThingsListViewModel{
			Things:  make([]components.ThingViewModel, 0),
			Pageing: getPaging(pageIndex_, pageLast, limit, result.Count, result.TotalRecords, offset, helpers.PagerIndexes(pageIndex_, pageLast), args),
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

		helpers.WriteComponentResponse(ctx, w, r, component, 1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewThingsTable(_ context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {

		ctx := helpers.Decorate(
			r.Context(),
			components.CurrentComponent, "things",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
		offset, limit := helpers.GetOffsetAndLimit(r)

		args := r.URL.Query()
		helpers.SanitizeParams(args, "mapview", "page", "limit", "offset")

		result, err := app.GetThings(ctx, offset, limit, args)
		if err != nil {
			http.Error(w, "could not fetch things", http.StatusInternalServerError)
			return
		}

		pageIndex_, _ := strconv.Atoi(pageIndex)
		pageLast := int(math.Ceil(float64(result.TotalRecords) / float64(limit)))

		model := components.ThingsListViewModel{
			Things:  make([]components.ThingViewModel, 0),
			Pageing: getPaging(pageIndex_, pageLast, limit, result.Count, result.TotalRecords, offset, helpers.PagerIndexes(pageIndex_, pageLast), args),
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

		helpers.WriteComponentResponse(ctx, w, r, component, 1024, 0)
	}

	return http.HandlerFunc(fn)
}

func toViewModel(thing application.Thing) components.ThingViewModel {
	tvm := components.ThingViewModel{
		ID:           thing.ID,
		Type:         thing.Type,
		SubType:      thing.SubType,
		Name:         thing.Name,
		Description:  thing.Description,
		Latitude:     thing.Location.Latitude,
		Longitude:    thing.Location.Longitude,
		Tenant:       thing.Tenant,
		Tags:         thing.Tags,
		ObservedAt:   thing.ObservedAt,
		Measurements: make([]components.MeasurementViewModel, 0),
		Properties:   make(map[string]any),
		RefDevice:    make([]string, 0),
	}

	for _, rd := range thing.RefDevices {
		tvm.RefDevice = append(tvm.RefDevice, rd.DeviceID)
	}

	if len(tvm.RefDevice) == 0 {
		tvm.RefDevice = append(tvm.RefDevice, "")
	}

	if len(thing.Values) > 0 {
		for _, m := range thing.Values[0] {
			vs := ""
			if m.StringValue != nil {
				vs = *m.StringValue
			}

			mvm := components.MeasurementViewModel{
				ID:          m.ID,
				Timestamp:   m.Timestamp,
				Urn:         m.Urn,
				BoolValue:   m.BoolValue,
				StringValue: vs,
				Unit:        m.Unit,
				Value:       m.Value,
			}

			tvm.Measurements = append(tvm.Measurements, mvm)
		}
	}

	for k, v := range toMap(thing.TypeValues) {
		if v != nil {
			tvm.Properties[k] = v
		}
	}

	return tvm
}

func toMap(v any) map[string]any {
	b, _ := json.Marshal(v)
	m := map[string]any{}
	_ = json.Unmarshal(b, &m)
	return m
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
		TargetURL:  "/components/tables/things",
		TargetID:   "#tableview",
	}
}
