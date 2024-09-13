package things
/*
import (
	"context"
	"math"
	"net/http"
	"strconv"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/locale"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

func NewTableThingsComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
	log := logging.GetFromContext(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")

		localizer := l10n.For(r.Header.Get("Accept-Language"))

		pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
		offset, limit := helpers.GetOffsetAndLimit(r)
		ctx := logging.NewContextWithLogger(r.Context(), log)

		listViewModel, totalRecords, err := composeViewModel(ctx, app, offset, limit)
		if err != nil {
			http.Error(w, "could not fetch things", http.StatusInternalServerError)
			return
		}

		pageIndex_, _ := strconv.Atoi(pageIndex)
		pageLast := float64(totalRecords) / float64(limit)

		ctx = helpers.Decorate(
			ctx,
			components.PageIndex, pageIndex_,
			components.PageLast, int(math.Ceil(pageLast)),
			components.PageSize, limit,
		)

		component := components.ThingTable(localizer, assets, *listViewModel)
		component.Render(ctx, w)

		w.WriteHeader(http.StatusOK)
	}

	return http.HandlerFunc(fn)
}
*/