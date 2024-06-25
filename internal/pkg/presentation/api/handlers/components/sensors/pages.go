package sensors

import (
	"context"
	"net/http"
	"strconv"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/locale"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
)

func NewSensorListPage(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
		offset, limit := helpers.GetOffsetAndLimit(r)

		ctx := helpers.Decorate(r.Context(),
			components.CurrentComponent, "sensors",
		)

		i, _ := strconv.Atoi(pageIndex)
		listViewModel, err := composeSensorListViewModel(ctx, offset, limit, i, app)
		if err != nil {
			http.Error(w, "could not fetch sensors", http.StatusInternalServerError)
			return
		}

		listViewModel.Meta.RawQuery = r.URL.RawQuery

		sensorList := components.Sensors(localizer, assets, *listViewModel)
		page := components.StartPage(version, localizer, assets, sensorList)

		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")

		ctx = helpers.Decorate(
			ctx,
			components.PageIndex, pageIndex,
			components.PageLast, listViewModel.Meta.TotalRecords/listViewModel.Meta.Limit,
			components.PageSize, listViewModel.Meta.Limit,
		)

		err = page.Render(ctx, w)
		if err != nil {
			http.Error(w, "could not render sensor details page", http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusOK)
	}

	return http.HandlerFunc(fn)
}

func NewSensorDetailsPage(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found i url", http.StatusBadRequest)
			return
		}

		ctx := helpers.Decorate(r.Context(),
			components.CurrentComponent, "sensors",
		)

		detailsViewModel, err := composeSensorDetailsViewModel(ctx, id, app)
		if err != nil {
			http.Error(w, "could not compose view model", http.StatusInternalServerError)
			return
		}

		sensorDetails := components.SensorDetailsPage(localizer, assets, *detailsViewModel)
		page := components.StartPage(version, localizer, assets, sensorDetails)

		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")

		err = page.Render(ctx, w)
		if err != nil {
			http.Error(w, "could not render sensor details page", http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusOK)
	}

	return http.HandlerFunc(fn)
}
