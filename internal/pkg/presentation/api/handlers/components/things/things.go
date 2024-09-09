package things
/*
import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

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

		listViewModel, totalRecords, err := composeViewModel(ctx, app, offset, limit)
		if err != nil {
			http.Error(w, "could not fetch things", http.StatusInternalServerError)
			return
		}

		thingList := components.Things(localizer, assets, *listViewModel)
		page := components.StartPage(version, localizer, assets, thingList)

		pageIndex_, _ := strconv.Atoi(pageIndex)
		pageLast := float64(totalRecords) / float64(limit)

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

func composeViewModel(ctx context.Context, app application.ThingManagement, offset, limit int) (*components.ThingListViewModel, int, error) {
	thingResult, err := app.GetThings(ctx, offset, limit, nil)
	if err != nil {
		return nil, 0, err
	}

	listViewModel := components.ThingListViewModel{}

	for _, thing := range thingResult.Things {
		thingID := thing.ThingID
		if thingID == "" {
			thingID = fmt.Sprintf("urn:diwise:%s:%s", strings.ToLower(thing.Type), strings.ToLower(thing.ID))
		}

		tvm := components.ThingViewModel{
			ThingID:      thingID,
			ID:           thing.ID,
			Type:         thing.Type,
			Latitude:     thing.Location.Latitude,
			Longitude:    thing.Location.Longitude,
			Tenant:       thing.Tenant,
			Measurements: make([]components.MeasurementViewModel, 0),
		}

		for _, measurement := range thing.Measurements {
			mvm := components.MeasurementViewModel{
				ID:          measurement.ID,
				Timestamp:   measurement.Timestamp,
				Urn:         measurement.Urn,
				BoolValue:   measurement.BoolValue,
				StringValue: measurement.StringValue,
				Unit:        measurement.Unit,
				Value:       measurement.Value,
			}
			tvm.Measurements = append(tvm.Measurements, mvm)
		}

		listViewModel.Things = append(listViewModel.Things, tvm)
	}

	return &listViewModel, thingResult.TotalRecords, nil
}

*/