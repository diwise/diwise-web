// Package catalog publishes thing templates and variants
// (iot-things-v2 /api/v1/catalog). Reading (list for dropdowns) stays on
// thingsv2.Service; this service owns version creation: published versions
// are immutable, so the GUI always copies a version and bumps it (PLAN002
// B3). Types are shared with thingsv2 (additive fields only).
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/diwise-web/internal/application/thingsv2"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("diwise-web/app/catalog")

type Service struct {
	client  *client.Client
	baseURL string
}

func NewService(client *client.Client, baseURL string) *Service {
	return &Service{client: client, baseURL: baseURL}
}

// GetTemplate reads one published template version (round-trip base for
// copy+bump; recipes ride along read-only).
func (s *Service) GetTemplate(ctx context.Context, tenant, id, version string) (thingsv2.TemplateSpec, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-template")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("tenant", tenant)

	body, _, err := s.client.GetRaw(ctx, s.baseURL, "catalog/templates/"+id+"/"+version, params)
	if err != nil {
		return thingsv2.TemplateSpec{}, err
	}

	var spec thingsv2.TemplateSpec
	if err = json.Unmarshal(body, &spec); err != nil {
		return thingsv2.TemplateSpec{}, fmt.Errorf("failed to decode template: %w", err)
	}

	return spec, nil
}

// PublishTemplate publishes a new template version. The backend answers
// 201 with empty body; duplicates surface as client.ErrConflict (B3:
// "versionen finns redan"), validation problems as plain errors carrying
// the backend text when available.
func (s *Service) PublishTemplate(ctx context.Context, tenant string, spec thingsv2.TemplateSpec) error {
	var err error
	ctx, span := tracer.Start(ctx, "publish-template")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("tenant", tenant)

	raw, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("failed to encode template: %w", err)
	}

	body, _, status, err := s.client.WriteRawDetailed(ctx, http.MethodPost, s.baseURL, "catalog/templates", params, nil, raw)
	if err != nil {
		return err
	}
	return checkStatus(body, status)
}

// GetVariant reads one published variant version.
func (s *Service) GetVariant(ctx context.Context, tenant, id, version string) (thingsv2.VariantSpec, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-variant")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("tenant", tenant)

	body, _, err := s.client.GetRaw(ctx, s.baseURL, "catalog/variants/"+id+"/"+version, params)
	if err != nil {
		return thingsv2.VariantSpec{}, err
	}

	var spec thingsv2.VariantSpec
	if err = json.Unmarshal(body, &spec); err != nil {
		return thingsv2.VariantSpec{}, fmt.Errorf("failed to decode variant: %w", err)
	}

	return spec, nil
}

// PublishVariant publishes a new variant version (same 201/409 semantics
// as templates).
func (s *Service) PublishVariant(ctx context.Context, tenant string, spec thingsv2.VariantSpec) error {
	var err error
	ctx, span := tracer.Start(ctx, "publish-variant")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("tenant", tenant)

	raw, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("failed to encode variant: %w", err)
	}

	body, _, status, err := s.client.WriteRawDetailed(ctx, http.MethodPost, s.baseURL, "catalog/variants", params, nil, raw)
	if err != nil {
		return err
	}
	return checkStatus(body, status)
}

func checkStatus(body []byte, status int) error {
	switch status {
	case http.StatusCreated, http.StatusOK, http.StatusNoContent:
		return nil
	case http.StatusBadRequest:
		if msg := errorMessage(body); msg != "" {
			return fmt.Errorf("invalid template: %s", msg)
		}
		return fmt.Errorf("request failed: %d", status)
	case http.StatusNotFound:
		return fmt.Errorf("request failed: %w", client.ErrNotFound)
	case http.StatusConflict:
		return fmt.Errorf("request failed: %w", client.ErrConflict)
	default:
		return fmt.Errorf("request failed: %d", status)
	}
}

func errorMessage(body []byte) string {
	var m struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &m); err != nil {
		return ""
	}
	return m.Error
}
