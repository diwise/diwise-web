package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/diwise/diwise-web/internal/application/devices"
	"github.com/diwise/diwise-web/internal/application/things"
	"github.com/diwise/diwise-web/internal/presentation/api/authz"
)

type sensorTenantResolverApp interface {
	GetDevice(ctx context.Context, id string) (devices.Device, error)
}

type thingTenantResolverApp interface {
	GetThing(ctx context.Context, id string, params map[string][]string) (things.Thing, error)
}

// NewTenantResolverFromSensorPath resolves a tenant from a sensor/device route path value.
// It expects routes shaped like /sensors/{id} or /components/sensors/{id}/...
func NewTenantResolverFromSensorPath(app sensorTenantResolverApp) authz.TenantResolver {
	return func(ctx context.Context, r *http.Request) (string, error) {
		id := strings.TrimSpace(r.PathValue("id"))
		if id == "" {
			return "", errors.New("missing sensor id")
		}

		device, err := app.GetDevice(ctx, id)
		if err != nil {
			return "", err
		}

		return device.Tenant, nil
	}
}

// NewTenantResolverFromThingsPath resolves a tenant from a thing route path value.
// It expects routes shaped like /things/{id} or /components/things/{id}/...
func NewTenantResolverFromThingsPath(app thingTenantResolverApp) authz.TenantResolver {
	return func(ctx context.Context, r *http.Request) (string, error) {
		id := strings.TrimSpace(r.PathValue("id"))
		if id == "" {
			return "", errors.New("missing thing id")
		}

		thing, err := app.GetThing(ctx, id, nil)
		if err != nil {
			return "", err
		}

		return thing.Tenant, nil
	}
}

// NewTenantResolverFromThingsQuery resolves a tenant from a thing id in the query string.
// It is intended for routes like /components/things/search-compatible-sensor-options?thingID=...
func NewTenantResolverFromThingsQuery(app thingTenantResolverApp) authz.TenantResolver {
	return func(ctx context.Context, r *http.Request) (string, error) {
		id := strings.TrimSpace(r.URL.Query().Get("thingID"))
		if id == "" {
			return "", errors.New("missing thing id")
		}

		thing, err := app.GetThing(ctx, id, nil)
		if err != nil {
			return "", err
		}

		return thing.Tenant, nil
	}
}
