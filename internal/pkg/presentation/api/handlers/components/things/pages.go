package things

import (
	"context"
	"net/http"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/locale"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
)

func NewThingsListPage(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
		offset, limit := helpers.GetOffsetAndLimit(r)

		ctx := helpers.Decorate(r.Context(),
			components.CurrentComponent, "things",
		)

		thingResult, err := app.GetThings(ctx, offset, limit)
		if err != nil {
			http.Error(w, "could not fetch things", http.StatusInternalServerError)
			return
		}

		sumOfStuff := app.GetStatistics(ctx)

		listViewModel := components.ThingListViewModel{
			Statistics: components.StatisticsViewModel{
				Total:    sumOfStuff.Total,
				Active:   sumOfStuff.Active,
				Inactive: sumOfStuff.Inactive,
				Online:   sumOfStuff.Online,
				Unknown:  sumOfStuff.Unknown,
			},
		}
		for _, thing := range thingResult.Things {
			listViewModel.Things = append(listViewModel.Things, components.ThingViewModel{
				Active:       thing.Active,
				ThingID:      thing.ThingID,
				DeviceID:     thing.DeviceID,
				Name:         thing.Name,
				BatteryLevel: thing.DeviceStatus.BatteryLevel,
				LastSeen:     thing.DeviceState.ObservedAt,
				HasAlerts:    false, //TODO: fix this
			})
		}

		thingList := components.Things(localizer, assets, listViewModel)
		page := components.StartPage(version, localizer, assets, thingList)

		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")

		renderCtx := helpers.Decorate(
			ctx,
			components.PageIndex, pageIndex,
			components.PageLast, thingResult.TotalRecords/limit,
			components.PageSize, limit,
		)

		err = page.Render(renderCtx, w)
		if err != nil {
			http.Error(w, "could not render thing details page", http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusOK)
	}

	return http.HandlerFunc(fn)
}

func composeViewModel(ctx context.Context, id string, app application.DeviceManagement) (*components.ThingDetailsViewModel, error) {
	thing, err := app.GetThing(ctx, id)
	if err != nil {
		return nil, err
	}

	tenants := app.GetTenants(ctx)
	//deviceProfiles := app.GetDeviceProfiles(ctx)

	/*tp := []components.DeviceProfile{}
	for _, p := range deviceProfiles {
		types := []string{}
		if p.Types != nil {
			types = *p.Types
		}
		tp = append(tp, components.DeviceProfile{
			Name:     p.Name,
			Decoder:  p.Decoder,
			Interval: p.Interval,
			Types:    types,
		})
	}*/

	types := []string{}
	for _, tp := range thing.Types {
		types = append(types, tp.URN)
	}

	/*measurements, err := app.GetMeasurementInfo(ctx, id)
	if err != nil {
		return nil, err
	}*/

	/*m := make([]string, 0)
	for _, md := range measurements.Measurements {
		m = append(m, md.ID)
	}*/

	detailsViewModel := components.ThingDetailsViewModel{
		ThingID:          thing.ThingID,
		Name:             thing.Name,
		Latitude:         thing.Location.Latitude,
		Longitude:        thing.Location.Longitude,
		ThingProfileName: thing.DeviceProfile.Name,
		Tenant:           thing.Tenant,
		Description:      thing.Description,
		Active:           thing.Active,
		Types:            types,
		Organisations:    tenants,
		//MeasurementTypes: m,
	}
	return &detailsViewModel, nil
}

func NewThingsDetailsPage(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		ctx := helpers.Decorate(r.Context(),
			components.CurrentComponent, "things",
		)

		detailsViewModel, err := composeViewModel(ctx, id, app)
		if err != nil {
			http.Error(w, "could not compose view model", http.StatusInternalServerError)
			return
		}

		thingDetails := components.ThingDetailsPage(localizer, assets, *detailsViewModel)
		page := components.StartPage(version, localizer, assets, thingDetails)

		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")

		err = page.Render(ctx, w)
		if err != nil {
			http.Error(w, "could not render thing details page", http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusOK)
	}

	return http.HandlerFunc(fn)
}
