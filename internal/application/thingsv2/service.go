package thingsv2

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

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

	body, _, err := s.client.GetRaw(ctx, s.baseURL, "things/"+id, params)
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

// GetHistory reads property history (oldest first) in [from, to].
// Empty property means all properties; empty from/to means unbounded.
func (s *Service) GetHistory(ctx context.Context, tenant, id, property string, from, to time.Time, limit int) ([]HistoryPoint, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-history-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("tenant", tenant)
	if property != "" {
		params.Add("property", property)
	}
	if !from.IsZero() {
		params.Add("from", from.UTC().Format(time.RFC3339))
	}
	if !to.IsZero() {
		params.Add("to", to.UTC().Format(time.RFC3339))
	}
	if limit > 0 {
		params.Add("limit", strconv.Itoa(limit))
	}

	body, _, err := s.client.GetRaw(ctx, s.baseURL, "things/"+id+"/history", params)
	if err != nil {
		return nil, err
	}

	var points []HistoryPoint
	if err = json.Unmarshal(body, &points); err != nil {
		return nil, fmt.Errorf("failed to decode history: %w", err)
	}
	if points == nil {
		points = []HistoryPoint{}
	}

	return points, nil
}

// GetBindings reads a thing's sensor bindings (device signals feeding
// its calculation inputs).
func (s *Service) GetBindings(ctx context.Context, tenant, id string) ([]Binding, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-bindings-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("tenant", tenant)

	body, _, err := s.client.GetRaw(ctx, s.baseURL, "things/"+id+"/bindings", params)
	if err != nil {
		return nil, err
	}

	var response struct {
		Bindings []Binding `json:"bindings"`
	}
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to decode bindings: %w", err)
	}
	if response.Bindings == nil {
		response.Bindings = []Binding{}
	}

	return response.Bindings, nil
}

// GetOverview reads a thing with its direct children (current values).
func (s *Service) GetOverview(ctx context.Context, tenant, id string) (Overview, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-overview-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("tenant", tenant)

	body, _, err := s.client.GetRaw(ctx, s.baseURL, "things/"+id+"/overview", params)
	if err != nil {
		return Overview{}, err
	}

	var overview Overview
	if err = json.Unmarshal(body, &overview); err != nil {
		return Overview{}, fmt.Errorf("failed to decode overview: %w", err)
	}
	if overview.Children == nil {
		overview.Children = []Thing{}
	}

	return overview, nil
}
