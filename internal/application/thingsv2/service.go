package thingsv2

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
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

	body, header, err := s.client.GetRaw(ctx, s.baseURL, "", s.params(tenant, f))
	if err != nil {
		return Result{}, err
	}

	var things []Thing
	if err = json.Unmarshal(body, &things); err != nil {
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

// ListThingsAcrossTenants fans out over the caller's tenants (iot-things-v2
// never merges tenants server-side) and merges sorted by thing ID.
// Failing tenants are skipped so one bad tenant never blanks the page;
// if every tenant fails the combined error is returned. No tenants means
// an empty result, not an error.
func (s *Service) ListThingsAcrossTenants(ctx context.Context, tenants []string, f Filter) (Result, error) {
	var err error
	ctx, span := tracer.Start(ctx, "list-things-v2-across-tenants")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	merged := []Thing{}
	total := 0
	var errs []error
	for _, tenant := range tenants {
		res, err := s.ListThings(ctx, tenant, f)
		if err != nil {
			errs = append(errs, fmt.Errorf("tenant %s: %w", tenant, err))
			continue
		}
		merged = append(merged, res.Things...)
		total += res.Total
	}
	if len(tenants) > 0 && len(errs) == len(tenants) {
		return Result{}, fmt.Errorf("all %d tenants failed: %v", len(tenants), errs)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].ThingID < merged[j].ThingID })
	return Result{Things: merged, Total: total}, nil
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
