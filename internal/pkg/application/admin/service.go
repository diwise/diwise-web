package admin

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/diwise/diwise-web/internal/pkg/application/common"
	"github.com/diwise/diwise-web/internal/pkg/application/devices"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("diwise-web/app/admin")

type Management interface {
	GetTenants(ctx context.Context) []string
	GetDeviceProfiles(ctx context.Context) []devices.SensorProfile
}

type Service struct {
	client *common.Client
}

func NewService(client *common.Client) *Service {
	return &Service{client: client}
}

func (s *Service) GetTenants(ctx context.Context) []string {
	var err error
	ctx, span := tracer.Start(ctx, "get-tenants")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	res, err := s.client.Get(ctx, s.client.AdminURL(), "tenants", url.Values{})
	if err != nil {
		return []string{}
	}

	var tenants []string
	if err = json.Unmarshal(res.Data, &tenants); err != nil {
		return []string{}
	}

	return tenants
}

func (s *Service) GetDeviceProfiles(ctx context.Context) []devices.SensorProfile {
	var err error
	ctx, span := tracer.Start(ctx, "get-deviceprofiles")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	res, err := s.client.Get(ctx, s.client.AdminURL(), "deviceprofiles", url.Values{})
	if err != nil {
		return []devices.SensorProfile{}
	}

	var profiles []devices.SensorProfile
	if err = json.Unmarshal(res.Data, &profiles); err != nil {
		return []devices.SensorProfile{}
	}

	return profiles
}
