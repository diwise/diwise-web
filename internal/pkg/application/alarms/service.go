package alarms

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/diwise/diwise-web/internal/pkg/application/common"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("diwise-web/app/alarms")

type Service struct {
	client *common.Client
}

func NewService(client *common.Client) *Service {
	return &Service{client: client}
}

func (s *Service) GetAlarms(ctx context.Context, offset, limit int, args map[string][]string) (Result, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-alarms")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("offset", fmt.Sprintf("%d", offset))
	params.Add("info", "true")
	for k, v := range args {
		params[k] = v
	}

	res, err := s.client.Get(ctx, s.client.AlarmsURL(), "", params)
	if err != nil {
		return Result{}, err
	}

	var alarms []Alarm
	err = json.Unmarshal(res.Data, &alarms)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Alarms:       alarms,
		TotalRecords: int(res.Meta.TotalRecords),
		Offset:       int(*res.Meta.Offset),
		Limit:        int(*res.Meta.Limit),
		Count:        len(alarms),
	}, nil
}
