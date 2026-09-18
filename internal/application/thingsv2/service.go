package thingsv2

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("diwise-web/app/thingsv2")

// TotalCountHeader carries the pre-paging total on list responses.
const TotalCountHeader = "X-Total-Count"

type Service struct {
	client  *client.Client
	baseURL string
}

func NewService(client *client.Client, baseURL string) *Service {
	return &Service{client: client, baseURL: baseURL}
}

func (s *Service) params(tenant string, f Filter) url.Values {
	params := url.Values{}
	params.Add("tenant", tenant)
	if f.Template != "" {
		params.Add("template", f.Template)
	}
	if f.TemplateVersion != "" {
		params.Add("templateVersion", f.TemplateVersion)
	}
	if f.Variant != "" {
		params.Add("variant", f.Variant)
	}
	if f.Name != "" {
		params.Add("name", f.Name)
	}
	if f.Category != "" {
		params.Add("category", f.Category)
	}
	if f.Limit > 0 {
		params.Add("limit", strconv.Itoa(f.Limit))
	}
	if f.Offset > 0 {
		params.Add("offset", strconv.Itoa(f.Offset))
	}
	return params
}

// ListThings lists one tenant's things, sorted by thing ID.
func (s *Service) ListThings(ctx context.Context, tenant string, f Filter) (Result, error) {
	var err error
	ctx, span := tracer.Start(ctx, "list-things-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	body, header, err := s.client.GetRaw(ctx, s.baseURL, "things", s.params(tenant, f))
	if err != nil {
		return Result{}, err
	}

	return decodeList(body, header)
}

// ListThingsAcrossTenants lists across all tenants the token grants access
// to: the server fans out (iot-things-v2 reads ?tenant= as a mere filter),
// so this is a single call without tenant selector. Results arrive merged
// and sorted with the pre-paging total.
func (s *Service) ListThingsAcrossTenants(ctx context.Context, f Filter) (Result, error) {
	var err error
	ctx, span := tracer.Start(ctx, "list-things-v2-across-tenants")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	body, header, err := s.client.GetRaw(ctx, s.baseURL, "things", s.params("", f))
	if err != nil {
		return Result{}, err
	}

	return decodeList(body, header)
}

// decodeList parses a list body with the pre-paging total from the header
// (falling back to page size when absent or invalid).
func decodeList(body []byte, header http.Header) (Result, error) {
	var things []Thing
	if err := json.Unmarshal(body, &things); err != nil {
		return Result{}, fmt.Errorf("failed to decode things: %w", err)
	}
	if things == nil {
		things = []Thing{}
	}

	total := len(things)
	if raw := header.Get(TotalCountHeader); raw != "" {
		if n, convErr := strconv.Atoi(raw); convErr == nil && n >= 0 {
			total = n
		}
	}

	return Result{Things: things, Total: total}, nil
}

// GetThing reads one thing with current values.
func (s *Service) GetThing(ctx context.Context, tenant, id string) (Thing, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-thing-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("tenant", tenant)

	body, _, err := s.client.GetRaw(ctx, s.baseURL, id, params)
	if err != nil {
		return Thing{}, err
	}

	var thing Thing
	if err = json.Unmarshal(body, &thing); err != nil {
		return Thing{}, fmt.Errorf("failed to decode thing: %w", err)
	}

	return thing, nil
}

// ListTemplates lists published template versions, optionally filtered
// by category (GUI dropdowns). A tenant is still required for auth.
func (s *Service) ListTemplates(ctx context.Context, tenant, category string) ([]TemplateSpec, error) {
	var err error
	ctx, span := tracer.Start(ctx, "list-templates-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("tenant", tenant)
	if category != "" {
		params.Add("category", category)
	}

	body, _, err := s.client.GetRaw(ctx, s.baseURL, "catalog/templates", params)
	if err != nil {
		return nil, err
	}

	var specs []TemplateSpec
	if err = json.Unmarshal(body, &specs); err != nil {
		return nil, fmt.Errorf("failed to decode templates: %w", err)
	}
	if specs == nil {
		specs = []TemplateSpec{}
	}

	return specs, nil
}
