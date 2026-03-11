package admin

import (
	"context"
	"net/http"
	"slices"
	"sort"
	"strings"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/authz"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	featureadmin "github.com/diwise/diwise-web/internal/pkg/presentation/web/components/features/admin"
	featureauth "github.com/diwise/diwise-web/internal/pkg/presentation/web/components/features/auth"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components/layout"
	shared "github.com/diwise/diwise-web/internal/pkg/presentation/web/components/shared"

	. "github.com/diwise/frontend-toolkit"
)

func NewMeasurementTypesComponentHandler(_ context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()

		localizer := l10n.For(r.Header.Get("Accept-Language"))

		sensorType := r.URL.Query().Get("sensorType")
		deviceProfiles := app.GetDeviceProfiles(ctx)

		i := slices.IndexFunc(deviceProfiles, func(p application.DeviceProfile) bool {
			return p.Decoder == sensorType
		})

		profile := deviceProfiles[i]

		options := []shared.OptionViewModel{}

		for _, t := range *profile.Types {
			parts := strings.Split(t, ":")
			text := strings.Join(parts[1:], "-")

			options = append(options, shared.OptionViewModel{
				Value:    t,
				Text:     localizer.Get(text),
				Name:     "measurementType-option[]",
				Selected: t == sensorType,
			})
		}

		sort.Slice(options, func(i int, j int) bool {
			return options[i].Text < options[j].Text
		})

		component := shared.CheckboxDropdownList("measurementType", options, localizer.Get("chooseMeasurementtype"))
		helpers.WriteComponentResponse(ctx, w, r, component, 2*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewErrorPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {

		ctx = helpers.Decorate(
			r.Context(),
			layout.CurrentComponent, "error",
		)
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		errorpage := featureauth.ErrorPage(localizer, assets)
		component := layout.StartPage(version, localizer, assets, errorpage)

		helpers.WriteComponentResponse(ctx, w, r, component, 30*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewAdminPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {

		ctx = helpers.Decorate(
			r.Context(),
			layout.CurrentComponent, "admin",
		)
		localizer := l10n.For(r.Header.Get("Accept-Language"))

		m := featureadmin.AdminViewModel{
			Token: authz.Token(ctx),
		}

		adminpage := featureadmin.AdminPage(localizer, assets, m)
		component := layout.StartPage(version, localizer, assets, adminpage)

		helpers.WriteComponentResponse(ctx, w, r, component, 30*1024, 0)
	}

	return http.HandlerFunc(fn)
}
