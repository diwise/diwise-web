package admin

import (
	"context"
	"net/http"
	"slices"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/locale"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
)

func NewMeasurementTypesComponentHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
		w.WriteHeader(http.StatusOK)

		ctx := r.Context()

		localizer := l10n.For(r.Header.Get("Accept-Language"))

		sensorType := r.URL.Query().Get("sensorType")
		deviceProfiles := app.GetDeviceProfiles(ctx)

		i := slices.IndexFunc(deviceProfiles, func(p application.DeviceProfile) bool {
			return p.Decoder == sensorType
		})

		profile := deviceProfiles[i]

		options := []components.OptionViewModel{}

		for _, t := range *profile.Types {
			options = append(options, components.OptionViewModel{
				Value:    t,
				Text:     localizer.Get(t),
				Name:     "measurementType-option[]",
				Selected: t == sensorType,
			})
		}

		component := components.OptionCheckboxes(options)
		component.Render(ctx, w)
	}

	return http.HandlerFunc(fn)
}
