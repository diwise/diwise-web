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
	if f.Tag != "" {
		params.Add("tag", f.Tag)
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

// ListTags lists distinct dynamic categorization tags across the token's
// tenants (server fans out like ListThingsAcrossTenants).
func (s *Service) ListTags(ctx context.Context) ([]string, error) {
	var err error
	ctx, span := tracer.Start(ctx, "list-tags-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	body, _, err := s.client.GetRaw(ctx, s.baseURL, "things/tags", url.Values{})
	if err != nil {
		return nil, err
	}

	var tags []string
	if err = json.Unmarshal(body, &tags); err != nil {
		return nil, fmt.Errorf("failed to decode tags: %w", err)
	}
	if tags == nil {
		tags = []string{}
	}

	return tags, nil
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

// ListVariants lists published variant versions.
func (s *Service) ListVariants(ctx context.Context, tenant string) ([]VariantSpec, error) {
	var err error
	ctx, span := tracer.Start(ctx, "list-variants-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("tenant", tenant)

	body, _, err := s.client.GetRaw(ctx, s.baseURL, "catalog/variants", params)
	if err != nil {
		return nil, err
	}

	var specs []VariantSpec
	if err = json.Unmarshal(body, &specs); err != nil {
		return nil, fmt.Errorf("failed to decode variants: %w", err)
	}
	if specs == nil {
		specs = []VariantSpec{}
	}

	return specs, nil
}

// GetConfig reads a thing's stored effective config (edit round-trip base).
func (s *Service) GetConfig(ctx context.Context, tenant, id string) (EffectiveConfig, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-config-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("tenant", tenant)

	body, _, err := s.client.GetRaw(ctx, s.baseURL, "things/"+id+"/config", params)
	if err != nil {
		return EffectiveConfig{}, err
	}

	var config EffectiveConfig
	if err = json.Unmarshal(body, &config); err != nil {
		return EffectiveConfig{}, fmt.Errorf("failed to decode config: %w", err)
	}

	return config, nil
}

// CreateThing creates a thing from an object spec.
func (s *Service) CreateThing(ctx context.Context, tenant string, spec ObjectSpec) (Thing, error) {
	var err error
	ctx, span := tracer.Start(ctx, "create-thing-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("tenant", tenant)

	raw, err := json.Marshal(spec)
	if err != nil {
		return Thing{}, fmt.Errorf("failed to encode thing: %w", err)
	}

	body, _, err := s.client.WriteRaw(ctx, http.MethodPost, s.baseURL, "things", params, nil, raw)
	if err != nil {
		return Thing{}, err
	}

	var thing Thing
	if err = json.Unmarshal(body, &thing); err != nil {
		return Thing{}, fmt.Errorf("failed to decode thing: %w", err)
	}

	return thing, nil
}

// UpdateThing replaces a thing from an object spec (revision is the
// config CAS stake; conflicts surface as client.ErrConflict).
func (s *Service) UpdateThing(ctx context.Context, tenant, id string, spec ObjectSpec, revision int64) (Thing, error) {
	var err error
	ctx, span := tracer.Start(ctx, "update-thing-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("tenant", tenant)

	raw, err := json.Marshal(spec)
	if err != nil {
		return Thing{}, fmt.Errorf("failed to encode thing: %w", err)
	}

	body, _, err := s.client.WriteRaw(ctx, http.MethodPut, s.baseURL, "things/"+id, params,
		map[string]string{"If-Match": fmt.Sprintf(`"rev-%d"`, revision)}, raw)
	if err != nil {
		return Thing{}, err
	}

	var thing Thing
	if err = json.Unmarshal(body, &thing); err != nil {
		return Thing{}, fmt.Errorf("failed to decode thing: %w", err)
	}

	return thing, nil
}

// DeleteThing deletes a thing (blocked with active children: ErrConflict).
func (s *Service) DeleteThing(ctx context.Context, tenant, id string) error {
	var err error
	ctx, span := tracer.Start(ctx, "delete-thing-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("tenant", tenant)

	_, _, err = s.client.WriteRaw(ctx, http.MethodDelete, s.baseURL, "things/"+id, params, nil, nil)
	return err
}

// MoveParent switches a thing's partOf parent atomically (revision is the
// config CAS stake; conflicts surface as client.ErrConflict).
func (s *Service) MoveParent(ctx context.Context, tenant, id, parentID string, revision int64) (Thing, error) {
	var err error
	ctx, span := tracer.Start(ctx, "move-parent-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("tenant", tenant)

	raw, err := json.Marshal(map[string]string{"parentId": parentID})
	if err != nil {
		return Thing{}, fmt.Errorf("failed to encode parent: %w", err)
	}

	body, _, err := s.client.WriteRaw(ctx, http.MethodPut, s.baseURL, "things/"+id+"/move", params,
		map[string]string{"If-Match": fmt.Sprintf(`"rev-%d"`, revision)}, raw)
	if err != nil {
		return Thing{}, err
	}

	var thing Thing
	if err = json.Unmarshal(body, &thing); err != nil {
		return Thing{}, fmt.Errorf("failed to decode thing: %w", err)
	}

	return thing, nil
}

// UnlinkParent detaches a thing's partOf parent without deleting the thing.
func (s *Service) UnlinkParent(ctx context.Context, tenant, id string, revision int64) (Thing, error) {
	var err error
	ctx, span := tracer.Start(ctx, "unlink-parent-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("tenant", tenant)

	body, _, err := s.client.WriteRaw(ctx, http.MethodDelete, s.baseURL, "things/"+id+"/parent", params,
		map[string]string{"If-Match": fmt.Sprintf(`"rev-%d"`, revision)}, nil)
	if err != nil {
		return Thing{}, err
	}

	var thing Thing
	if err = json.Unmarshal(body, &thing); err != nil {
		return Thing{}, fmt.Errorf("failed to decode thing: %w", err)
	}

	return thing, nil
}

// SwapBindings replaces a thing's sensor bindings atomically (revision is
// the config CAS stake; conflicts surface as client.ErrConflict).
func (s *Service) SwapBindings(ctx context.Context, tenant, id string, bindings []Binding, revision int64) (Thing, error) {
	var err error
	ctx, span := tracer.Start(ctx, "swap-bindings-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("tenant", tenant)

	raw, err := json.Marshal(map[string][]Binding{"bindings": bindings})
	if err != nil {
		return Thing{}, fmt.Errorf("failed to encode bindings: %w", err)
	}

	body, _, err := s.client.WriteRaw(ctx, http.MethodPut, s.baseURL, "things/"+id+"/bindings", params,
		map[string]string{"If-Match": fmt.Sprintf(`"rev-%d"`, revision)}, raw)
	if err != nil {
		return Thing{}, err
	}

	var thing Thing
	if err = json.Unmarshal(body, &thing); err != nil {
		return Thing{}, fmt.Errorf("failed to decode thing: %w", err)
	}

	return thing, nil
}
