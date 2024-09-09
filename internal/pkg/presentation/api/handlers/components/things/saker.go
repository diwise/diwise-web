package things

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/locale"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
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

		// remove args with values in template
		args := r.URL.Query()
		helpers.SanitizeParams(args, "page", "limit", "offset")

		result, err := app.GetThings(ctx, offset, limit, args)
		if err != nil {
			http.Error(w, "could not fetch things", http.StatusInternalServerError)
			return
		}

		pageIndex_, _ := strconv.Atoi(pageIndex)
		pageLast := float64(result.TotalRecords) / float64(limit)

		model := components.ThingsListViewModel{
			Things: make([]components.ThingViewModel, 0),
			Pageing: components.PagingViewModel{
				PageIndex: pageIndex_,
				PageLast:  int(math.Ceil(pageLast)),
				PageSize:  limit,
				Offset:    offset,
				Pages:     helpers.PagerIndexes(pageIndex_, int(math.Ceil(pageLast))),
				Query:     args.Encode(),
				TargetURL: "/components/tables/things",
				TargetID:  "#tableview",
			},
		}

		for _, thing := range result.Things {
			tvm := components.ThingViewModel{
				ThingID:   thing.ThingID,
				ID:        thing.ID,
				Type:      thing.Type,
				Latitude:  thing.Location.Latitude,
				Longitude: thing.Location.Longitude,
				Tenant:    thing.Tenant,
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

			model.Things = append(model.Things, tvm)
		}

		component := components.ThingsTable(localizer, model)

		ctx = helpers.Decorate(
			ctx,
			components.PageIndex, pageIndex_,
			components.PageLast, int(math.Ceil(pageLast)),
			components.PageSize, limit,
		)

		//templ.Handler(component, templ.WithStreaming()).ServeHTTP(w, r)

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

		args := r.URL.Query()
		helpers.SanitizeParams(args, "page", "limit", "offset")

		result, err := app.GetThings(ctx, offset, limit, args)
		if err != nil {
			http.Error(w, "could not fetch things", http.StatusInternalServerError)
			return
		}

		pageIndex_, _ := strconv.Atoi(pageIndex)
		pageLast := float64(result.TotalRecords) / float64(limit)

		model := components.ThingsListViewModel{
			Things: make([]components.ThingViewModel, 0),
			Pageing: components.PagingViewModel{
				PageIndex: pageIndex_,
				PageLast:  int(math.Ceil(pageLast)),
				PageSize:  limit,
				Offset:    offset,
				Pages:     helpers.PagerIndexes(pageIndex_, int(math.Ceil(pageLast))),
				Query:     args.Encode(),
				TargetURL: "/components/tables/saker",
				TargetID:  "#things-table",
			},
		}

		for _, thing := range result.Things {
			tvm := components.ThingViewModel{
				ThingID:   thing.ThingID,
				ID:        thing.ID,
				Type:      thing.Type,
				Latitude:  thing.Location.Latitude,
				Longitude: thing.Location.Longitude,
				Tenant:    thing.Tenant,
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

			model.Things = append(model.Things, tvm)
		}

		thingList := components.ThingsList(localizer, model)
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
