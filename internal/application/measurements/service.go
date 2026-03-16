package measurements

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("diwise-web/app/measurements")

type Service struct {
	client *client.Client
}

func NewService(client *client.Client) *Service {
	return &Service{client: client}
}

func (s *Service) GetMeasurementInfo(ctx context.Context, id string) ([]Value, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-measurementinfo")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	q := url.Values{}
	q.Add("latest", "true")

	resp, err := s.client.Get(ctx, s.client.MeasurementURL(), id, q)
	if err != nil {
		return []Value{}, err
	}

	var info []Value
	err = json.Unmarshal(resp.Data, &info)
	if err != nil {
		return []Value{}, err
	}

	return info, nil
}

func (s *Service) GetMeasurementData(ctx context.Context, id string, params ...client.InputParam) (Data, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-measurementdata")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	q := url.Values{}
	if id != "" {
		q.Add("id", id)
	}

	for _, p := range params {
		p(&q)
	}

	resp, err := s.client.Get(ctx, s.client.MeasurementURL(), "", q)
	if err != nil {
		return Data{}, err
	}

	var data Data
	err = json.Unmarshal(resp.Data, &data)
	if err != nil {
		return Data{}, err
	}

	return data, nil
}
