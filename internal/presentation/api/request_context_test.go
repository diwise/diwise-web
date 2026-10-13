package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/diwise/diwise-web/internal/application/alarms"
	"github.com/diwise/diwise-web/internal/application/devices"
	"github.com/diwise/diwise-web/internal/application/measurements"
	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	"github.com/diwise/diwise-web/internal/presentation/api/handlers/admin"
	"github.com/diwise/diwise-web/internal/presentation/api/handlers/home"
	frontend "github.com/diwise/frontend-toolkit"
	ftkmock "github.com/diwise/frontend-toolkit/mock"
)

type homeDevices interface{ devices.Management }
type homeMeasurements interface{ measurements.Management }

type contextHomeApp struct {
	homeDevices
	homeMeasurements
}

func (contextHomeApp) GetAlarms(ctx context.Context, _, _ int, _ map[string][]string) (alarms.Result, error) {
	return alarms.Result{Alarms: []alarms.Alarm{{DeviceID: auth.Token(ctx)}}}, nil
}

func TestConcurrentPageRequestsKeepTheirOwnToken(t *testing.T) {
	l10n := &ftkmock.LocaleBundleMock{ForFunc: func(string) frontend.Localizer {
		return &ftkmock.LocalizerMock{
			GetFunc:         func(key string) string { return key },
			GetWithDataFunc: func(key string, _ map[string]any) string { return key },
		}
	}}
	for name, h := range map[string]http.Handler{
		"admin": admin.NewAdminPage(context.Background(), l10n, nil, nil),
		"home":  home.NewHomePage(context.Background(), l10n, nil, contextHomeApp{}),
	} {
		t.Run(name, func(t *testing.T) {
			var wg sync.WaitGroup
			start := make(chan struct{})
			for i := range 32 {
				wg.Go(func() {
					token := fmt.Sprintf("request-token-%02d-end", i)
					r := httptest.NewRequest(http.MethodGet, "/"+name, nil)
					r.Header.Set("HX-Request", "true")
					r = r.WithContext(auth.WithToken(r.Context(), token))
					w := httptest.NewRecorder()
					<-start
					h.ServeHTTP(w, r)
					if !strings.Contains(w.Body.String(), token) {
						t.Errorf("%s response lost its own token", token)
					}
					for j := range 32 {
						if j != i && strings.Contains(w.Body.String(), fmt.Sprintf("request-token-%02d-end", j)) {
							t.Errorf("%s response contains another request's token", token)
						}
					}
				})
			}
			close(start)
			wg.Wait()
		})
	}
}
