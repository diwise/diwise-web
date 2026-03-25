package admin

import (
	"context"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/authz"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	featureadmin "github.com/diwise/diwise-web/internal/pkg/presentation/webv2/components/features/admin"
	v2layout "github.com/diwise/diwise-web/internal/pkg/presentation/webv2/components/layout"
	. "github.com/diwise/frontend-toolkit"
)

func NewAdminPage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, _ *application.App) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx = helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "admin",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		model := featureadmin.AdminViewModel{
			Token: authz.Token(ctx),
		}

		adminPage := featureadmin.AdminPage(localizer, model)
		component := templ.Component(v2layout.StartPage(version, localizer, assets, adminPage))
		if helpers.IsHxRequest(r) {
			component = v2layout.AppShell(localizer, assets, adminPage)
		}

		helpers.WriteComponentResponse(ctx, w, r, component, 30*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewImportHandler(_ context.Context, _ LocaleBundle, _ AssetLoaderFunc, app *application.App) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		defer r.Body.Close()

		contentType := r.Header.Get("Content-Type")
		if !strings.Contains(contentType, "multipart/form-data") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		f, _, err := r.FormFile("file")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer f.Close()

		importType := r.FormValue("type")
		if err := app.Import(ctx, importType, f); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		http.Redirect(w, r, "/v2/admin", http.StatusSeeOther)
	}

	return http.HandlerFunc(fn)
}
