package home

import (
	"context"
	"net/http"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/locale"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
)

func NewOverviewCardsHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		//w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
		w.WriteHeader(http.StatusOK)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		ctx = r.Context()
		stats := app.GetStatistics(ctx)

		component := components.OverviewCards(localizer, assets, components.StatisticsViewModel{
			Total: stats.Total,
			Active: stats.Active,
			Inactive: stats.Inactive,
			Online: stats.Online,
			Unknown: stats.Unknown,
		})

		component.Render(ctx, w)
	}
	return http.HandlerFunc(fn)
}

func NewHomePage(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		//w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
		w.WriteHeader(http.StatusOK)

		ctx = helpers.Decorate(
			r.Context(),
			components.CurrentComponent, "home",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))

		component := components.StartPage(
			version, localizer,
			assets, components.Home(localizer, assets),
		)

		component.Render(ctx, w)
	}

	return http.HandlerFunc(fn)
}
