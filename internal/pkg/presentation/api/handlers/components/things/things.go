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

func NewThingsPage(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {

		ctx = helpers.Decorate(
			r.Context(),
			components.CurrentComponent, "things",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))

		pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
		offset, limit := helpers.GetOffsetAndLimit(r)

		thingResult, err := app.GetThings(ctx, offset, limit)
		if err != nil {
			http.Error(w, "could not fetch things", http.StatusInternalServerError)
			return
		}

		listViewModel := components.ThingListViewModel{}

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
