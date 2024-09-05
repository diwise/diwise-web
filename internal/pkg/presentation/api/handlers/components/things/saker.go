package things

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/locale"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components/ui"
)

func NewSakerTable(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
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

		result, err := app.GetThings(ctx, offset, limit)
		if err != nil {
			http.Error(w, "could not fetch things", http.StatusInternalServerError)
			return
		}

		pageIndex_, _ := strconv.Atoi(pageIndex)
		pageLast := float64(result.TotalRecords) / float64(limit)

		// remove args with values in template
		args := r.URL.Query()
		sanitizeParams(args, "page", "limit", "offset")

		model := ui.ThingsListViewModel{
			Things: make([]ui.ThingViewModel, 0),
			Pageing: ui.PagingViewModel{
				PageIndex: pageIndex_,
				PageLast:  int(math.Ceil(pageLast)),
				PageSize:  limit,
				Offset:    offset,
				Pages:     pagerIndexes(pageIndex_, int(math.Ceil(pageLast))),
				Query:     args.Encode(),
			},
		}

		for _, thing := range result.Things {
			tvm := ui.ThingViewModel{
				ThingID:   thing.ThingID,
				ID:        thing.ID,
				Type:      thing.Type,
				Latitude:  thing.Location.Latitude,
				Longitude: thing.Location.Longitude,
				Tenant:    thing.Tenant,
			}

			for _, m := range thing.Measurements {
				mvm := ui.MeasurementViewModel{
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

			model.Things = append(model.Things, tvm)
		}

		component := ui.ThingsTable(localizer, model)

		ctx = helpers.Decorate(
			ctx,
			components.PageIndex, pageIndex_,
			components.PageLast, int(math.Ceil(pageLast)),
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

func NewSakerPage(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
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

		result, err := app.GetThings(ctx, offset, limit)
		if err != nil {
			http.Error(w, "could not fetch things", http.StatusInternalServerError)
			return
		}

		pageIndex_, _ := strconv.Atoi(pageIndex)
		pageLast := float64(result.TotalRecords) / float64(limit)

		args := r.URL.Query()
		sanitizeParams(args, "page", "limit", "offset")

		model := ui.ThingsListViewModel{
			Things: make([]ui.ThingViewModel, 0),
			Pageing: ui.PagingViewModel{
				PageIndex: pageIndex_,
				PageLast:  int(math.Ceil(pageLast)),
				PageSize:  limit,
				Offset:    offset,
				Pages:     pagerIndexes(pageIndex_, int(math.Ceil(pageLast))),
				Query:     args.Encode(),
			},
		}

		for _, thing := range result.Things {
			tvm := ui.ThingViewModel{
				ThingID:   thing.ThingID,
				ID:        thing.ID,
				Type:      thing.Type,
				Latitude:  thing.Location.Latitude,
				Longitude: thing.Location.Longitude,
				Tenant:    thing.Tenant,
			}

			for _, m := range thing.Measurements {
				mvm := ui.MeasurementViewModel{
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

			model.Things = append(model.Things, tvm)
		}

		thingList := ui.ThingsList(localizer, model)
		page := components.StartPage(version, localizer, assets, thingList)

		ctx = helpers.Decorate(
			ctx,
			components.PageIndex, pageIndex_,
			components.PageLast, int(math.Ceil(pageLast)),
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

func pagerIndexes(pageIndex, pageCount int) []int64 {
	start := int64(pageIndex)
	last := int64(pageCount)

	const PagerWidth int64 = 6

	start -= (PagerWidth / 2)

	if start > (last - PagerWidth) {
		start = last - PagerWidth
	}

	if start < 1 {
		start = 1
	}

	result := []int64{}

	if start != 1 {
		start = start + 1
		result = append(result, 1, start)
	} else {
		result = append(result, 1)
	}

	page := start + 1

	for len(result) < int(PagerWidth) {
		if page >= last {
			break
		}

		result = append(result, page)
		page = page + 1
	}

	if result[len(result)-1] < last {
		result = append(result, last)
	}

	return result
}

func sanitizeParams(params url.Values, keys ...string) {
	if len(keys) > 0 {
		for _, k := range keys {
			params.Del(k)
		}
	}

	for k, v := range params {
		for i := 0; i < len(v); i++ {
			if v[i] == "" {
				v = append(v[:i], v[i+1:]...)
				i--
			}
		}

		for i := 0; i < len(v); i++ {
			for j := i + 1; j < len(v); j++ {
				if v[i] == v[j] {
					v = append(v[:j], v[j+1:]...)
					j--
				}
			}
		}

		if len(v) == 0 {
			params.Del(k)
		} else {
			params[k] = v
		}
	}
}
