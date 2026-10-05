package transform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("diwise-web/app/transform")

// ValidationError carries a backend 400/422 text (rule validation) for form
// display. The backend responds {error: string} in cleartext. Wraps
// client.ErrBadRequest (400) or client.ErrUnprocessable (422) so
// errors.Is classifies it.
type ValidationError struct {
	Message string
	Status  int
}

func (e *ValidationError) Error() string { return "invalid rule: " + e.Message }

func (e *ValidationError) Unwrap() error {
	if e.Status == http.StatusUnprocessableEntity {
		return client.ErrUnprocessable
	}
	return client.ErrBadRequest
}

// Service is the client for iot-transform-fiware (/api/v0): rule CRUD
// plus dry-run validate/preview. Writes carry If-Match rev-n; conflicts
// surface as client.ErrConflict, unknown ids as client.ErrNotFound.
type Service struct {
	client  *client.Client
	baseURL string
}

func NewService(client *client.Client, baseURL string) *Service {
	return &Service{client: client, baseURL: baseURL}
}

// Configured reports whether a backend URL is set (empty TRANSFORM_URL
// means the rules pages render "not configured" instead of calling out).
func (s *Service) Configured() bool {
	return s.baseURL != ""
}

func tenantParams(tenant string, filters map[string]string) url.Values {
	params := url.Values{}
	params.Add("tenant", tenant)
	for k, v := range filters {
		if v != "" {
			params.Add(k, v)
		}
	}
	return params
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

// ListModels lists visible rules in precedence order, optionally filtered
// (kind/event/type/tenant/source — server-side AND, unknown values match
// nothing). A tenant is still required for auth.
func (s *Service) ListModels(ctx context.Context, tenant string, filters map[string]string) ([]Model, error) {
	var err error
	ctx, span := tracer.Start(ctx, "list-models")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	body, _, err := s.client.GetRaw(ctx, s.baseURL, "models", tenantParams(tenant, filters))
	if err != nil {
		return nil, err
	}

	var out struct {
		Models []Model `json:"models"`
	}
	if err = json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("failed to decode models: %w", err)
	}
	if out.Models == nil {
		out.Models = []Model{}
	}

	return out.Models, nil
}

// GetModel reads one rule with its revision.
func (s *Service) GetModel(ctx context.Context, tenant, id string) (Model, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-model")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	if err = client.ValidatePathID(id, "model"); err != nil {
		return Model{}, err
	}

	params := url.Values{}
	params.Add("tenant", tenant)

	body, _, err := s.client.GetRaw(ctx, s.baseURL, "models/"+id, params)
	if err != nil {
		return Model{}, err
	}

	var model Model
	if err = json.Unmarshal(body, &model); err != nil {
		return Model{}, fmt.Errorf("failed to decode model: %w", err)
	}

	return model, nil
}

// CreateModel creates a rule (tenant from rule.match.tenant, must be
// within authorized tenants; global rules are seed-administered).
func (s *Service) CreateModel(ctx context.Context, tenant string, rule Rule) (Model, error) {
	var err error
	ctx, span := tracer.Start(ctx, "create-model")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	raw, err := json.Marshal(rule)
	if err != nil {
		return Model{}, fmt.Errorf("failed to encode rule: %w", err)
	}

	params := url.Values{}
	params.Add("tenant", tenant)

	body, _, status, err := s.client.WriteRawDetailed(ctx, http.MethodPost, s.baseURL, "models", params, nil, raw)
	if err != nil {
		return Model{}, err
	}
	return decodeModelResponse(body, status)
}

// UpdateModel replaces a rule (revision is the CAS stake; conflicts
// surface as client.ErrConflict). Overwriting a seed row flips its source
// to api — callers confirm takeover first (PLAN002 B4).
func (s *Service) UpdateModel(ctx context.Context, tenant, id string, rule Rule, revision int64) (Model, error) {
	var err error
	ctx, span := tracer.Start(ctx, "update-model")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	if err = client.ValidatePathID(id, "model"); err != nil {
		return Model{}, err
	}

	raw, err := json.Marshal(rule)
	if err != nil {
		return Model{}, fmt.Errorf("failed to encode rule: %w", err)
	}

	params := url.Values{}
	params.Add("tenant", tenant)

	body, _, status, err := s.client.WriteRawDetailed(ctx, http.MethodPut, s.baseURL, "models/"+id, params,
		map[string]string{"If-Match": fmt.Sprintf(`"rev-%d"`, revision)}, raw)
	if err != nil {
		return Model{}, err
	}
	return decodeModelResponse(body, status)
}

// DeleteModel deletes a rule (revision is the CAS stake).
func (s *Service) DeleteModel(ctx context.Context, tenant, id string, revision int64) error {
	var err error
	ctx, span := tracer.Start(ctx, "delete-model")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	if err = client.ValidatePathID(id, "model"); err != nil {
		return err
	}

	params := url.Values{}
	params.Add("tenant", tenant)

	_, _, status, err := s.client.WriteRawDetailed(ctx, http.MethodDelete, s.baseURL, "models/"+id, params,
		map[string]string{"If-Match": fmt.Sprintf(`"rev-%d"`, revision)}, nil)
	if err != nil {
		return err
	}
	return checkWriteStatus(status)
}

// ValidateRule dry-validates a rule without saving.
func (s *Service) ValidateRule(ctx context.Context, tenant string, rule Rule) error {
	var err error
	ctx, span := tracer.Start(ctx, "validate-rule")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	raw, err := json.Marshal(rule)
	if err != nil {
		return fmt.Errorf("failed to encode rule: %w", err)
	}

	params := url.Values{}
	params.Add("tenant", tenant)

	body, _, status, err := s.client.WriteRawDetailed(ctx, http.MethodPost, s.baseURL, "models/validate", params, nil, raw)
	if err != nil {
		return err
	}
	if status == http.StatusOK {
		return nil
	}
	if msg := errorMessage(body); msg != "" && (status == http.StatusBadRequest || status == http.StatusUnprocessableEntity) {
		return &ValidationError{Message: msg, Status: status}
	}
	return checkWriteStatus(status)
}

// NewPreviewEvent builds a preview event from raw JSON text, enforcing
// exactly one JSON document (same rule as backend decodeJSONBody). Returns
// RawMessage so no field is lost in roundtrip.
func NewPreviewEvent(kind, rawMessage, sensorType string) (PreviewEvent, error) {
	if kind == "" {
		return PreviewEvent{}, &ValidationError{Message: "preview kind is required", Status: http.StatusBadRequest}
	}
	trimmed := []byte(rawMessage)
	if len(trimmed) == 0 {
		return PreviewEvent{}, &ValidationError{Message: "preview message is required", Status: http.StatusBadRequest}
	}
	if !json.Valid(trimmed) {
		return PreviewEvent{}, &ValidationError{Message: "preview message must be exactly one JSON document", Status: http.StatusBadRequest}
	}
	// Exactly one document: a second Decode must hit EOF.
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	var first json.RawMessage
	if err := dec.Decode(&first); err != nil {
		return PreviewEvent{}, &ValidationError{Message: "preview message must be exactly one JSON document", Status: http.StatusBadRequest}
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return PreviewEvent{}, &ValidationError{Message: "preview message must be exactly one JSON document", Status: http.StatusBadRequest}
	}
	return PreviewEvent{Kind: kind, Message: json.RawMessage(trimmed), SensorType: sensorType}, nil
}

// PreviewRule dry-runs an inline rule against an example event. Message must
// be exactly one JSON document (use NewPreviewEvent); empty/invalid yields a
// client-side ValidationError without backend contact. Never triggers broker
// calls or mutations.
func (s *Service) PreviewRule(ctx context.Context, tenant string, rule Rule, event PreviewEvent) (PreviewResult, error) {
	var err error
	ctx, span := tracer.Start(ctx, "preview-rule")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	if event.Kind == "" {
		return PreviewResult{}, &ValidationError{Message: "preview kind is required", Status: http.StatusBadRequest}
	}
	if len(event.Message) == 0 {
		return PreviewResult{}, &ValidationError{Message: "preview message is required", Status: http.StatusBadRequest}
	}
	if !json.Valid(event.Message) {
		return PreviewResult{}, &ValidationError{Message: "preview message must be exactly one JSON document", Status: http.StatusBadRequest}
	}

	req := struct {
		Rule  Rule         `json:"rule"`
		Event PreviewEvent `json:"event"`
	}{Rule: rule, Event: event}
	raw, err := json.Marshal(req)
	if err != nil {
		return PreviewResult{}, fmt.Errorf("failed to encode preview: %w", err)
	}

	params := url.Values{}
	params.Add("tenant", tenant)

	body, _, status, err := s.client.WriteRawDetailed(ctx, http.MethodPost, s.baseURL, "models/preview", params, nil, raw)
	if err != nil {
		return PreviewResult{}, err
	}
	if status != http.StatusOK {
		if msg := errorMessage(body); msg != "" && (status == http.StatusBadRequest || status == http.StatusUnprocessableEntity) {
			return PreviewResult{}, &ValidationError{Message: msg, Status: status}
		}
		return PreviewResult{}, checkWriteStatus(status)
	}

	var res PreviewResult
	if err = json.Unmarshal(body, &res); err != nil {
		return PreviewResult{}, fmt.Errorf("failed to decode preview: %w", err)
	}
	res = normalizePreviewResult(res)

	return res, nil
}

// normalizePreviewResult keeps E wire "competing" verbatim and maps it to
// GUI views "matches"/"diagnostics" (A-rekommendation Våg3: backend behåller
// competing, G mappar + dokumenterar). Skipped stays []Diagnostic with full
// fields (code,ruleId,ruleRevision,snapshotVersion,entity,attribute,
// expression,tenant,thingId,configRevision) without loss; legacy
// reason/target/message keys survive in Diagnostic's tolerant aliases.
// Old stubs without competing (matches/diagnostics only) derive competing
// so J-acceptans sees one canonical view either way.
func normalizePreviewResult(res PreviewResult) PreviewResult {
	if res.Entities == nil {
		res.Entities = []PreviewEntity{}
	}
	if res.Competing == nil {
		res.Competing = []MatchDiagnostic{}
	}
	if res.Skipped == nil {
		res.Skipped = []Diagnostic{}
	}
	if res.Matches == nil {
		res.Matches = []PreviewMatch{}
	}
	if res.Diagnostics == nil {
		res.Diagnostics = []MatchDiagnostic{}
	}
	if res.Overlaps == nil {
		res.Overlaps = []RuleOverlap{}
	}
	if len(res.Competing) > 0 {
		if len(res.Matches) == 0 {
			res.Matches = CompetingToMatches(res.Competing)
		}
		if len(res.Diagnostics) == 0 {
			res.Diagnostics = append([]MatchDiagnostic(nil), res.Competing...)
		}
	} else {
		if len(res.Matches) > 0 {
			res.Competing = MatchesToCompeting(res.Matches)
		} else if len(res.Diagnostics) > 0 {
			res.Competing = append([]MatchDiagnostic(nil), res.Diagnostics...)
			res.Matches = CompetingToMatches(res.Competing)
		}
		if len(res.Diagnostics) == 0 && len(res.Competing) > 0 {
			res.Diagnostics = append([]MatchDiagnostic(nil), res.Competing...)
		}
		if len(res.Matches) == 0 && len(res.Competing) > 0 {
			res.Matches = CompetingToMatches(res.Competing)
		}
	}
	return res
}

// CheckCompatibility checks a candidate rule set against an explicit
// template version + source contract (D-facit CompatRequest{template,
// sources,rules,tenant,thingId,configRevision} + POST /models/compat,
// register.go + compat_d_test.go). Tenant in body never widens token auth:
// query ?tenant= carries auth scope, body tenant/thingId/configRevision are
// diagnostic context only. Empty template version/variant is wildcard per D
// (old G required version; now only thingType is required). Response is
// CompatResult{compatible,diagnostics}; Issues/Warnings are derived for H/I
// templ. A 404 from an old backend surfaces as ErrNotFound (caller falls
// back to ValidateRule).
func (s *Service) CheckCompatibility(ctx context.Context, tenant string, req CompatibilityRequest) (CompatibilityResult, error) {
	var err error
	ctx, span := tracer.Start(ctx, "check-compatibility")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	effectiveThingType := req.Template.ThingType
	if effectiveThingType == "" {
		effectiveThingType = req.TemplateID
	}
	if effectiveThingType == "" {
		return CompatibilityResult{}, fmt.Errorf("template thingType is required: %w", client.ErrBadRequest)
	}
	if len(req.Rules) == 0 {
		return CompatibilityResult{}, fmt.Errorf("at least one rule is required: %w", client.ErrBadRequest)
	}
	// Body tenant defaults to query tenant for diagnostic context; never
	// widens token auth (backend checks auth context, not body).
	if req.Tenant == "" {
		req.Tenant = tenant
	}

	raw, err := json.Marshal(req)
	if err != nil {
		return CompatibilityResult{}, fmt.Errorf("failed to encode compatibility: %w", err)
	}

	params := url.Values{}
	params.Add("tenant", tenant)

	body, _, status, err := s.client.WriteRawDetailed(ctx, http.MethodPost, s.baseURL, "models/compat", params, nil, raw)
	if err != nil {
		return CompatibilityResult{}, err
	}
	switch status {
	case http.StatusOK:
		var res CompatibilityResult
		if err = json.Unmarshal(body, &res); err != nil {
			return CompatibilityResult{}, fmt.Errorf("failed to decode compatibility: %w", err)
		}
		res = normalizeCompatibilityResult(res)
		return res, nil
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		if msg := errorMessage(body); msg != "" {
			return CompatibilityResult{}, &ValidationError{Message: msg, Status: status}
		}
		return CompatibilityResult{}, checkWriteStatus(status)
	case http.StatusNotFound:
		return CompatibilityResult{}, fmt.Errorf("request failed: %w", client.ErrNotFound)
	default:
		return CompatibilityResult{}, checkWriteStatus(status)
	}
}

// normalizeCompatibilityResult keeps D diagnostics verbatim and derives
// legacy GUI views Issues/Warnings for H/I templ (which ranges over them).
// Old stubs with issues/warnings only derive diagnostics back so J sees one
// canonical view either way. No diagnostic value is dropped: Detail/Message,
// Entity/TargetRole, Attribute/Target survive in both shapes.
func normalizeCompatibilityResult(res CompatibilityResult) CompatibilityResult {
	if res.Diagnostics == nil {
		res.Diagnostics = []Diagnostic{}
	}
	if res.Issues == nil {
		res.Issues = []CompatibilityIssue{}
	}
	if res.Warnings == nil {
		res.Warnings = []CompatibilityIssue{}
	}
	if len(res.Diagnostics) > 0 && len(res.Issues) == 0 {
		for _, d := range res.Diagnostics {
			res.Issues = append(res.Issues, DiagnosticToIssue(d))
		}
	}
	if len(res.Diagnostics) == 0 && (len(res.Issues) > 0 || len(res.Warnings) > 0) {
		for _, i := range res.Issues {
			res.Diagnostics = append(res.Diagnostics, IssueToDiagnostic(i))
		}
		for _, w := range res.Warnings {
			res.Diagnostics = append(res.Diagnostics, IssueToDiagnostic(w))
		}
	}
	return res
}

func decodeModelResponse(body []byte, status int) (Model, error) {
	switch status {
	case http.StatusOK, http.StatusCreated:
		var model Model
		if err := json.Unmarshal(body, &model); err != nil {
			return Model{}, fmt.Errorf("failed to decode model: %w", err)
		}
		return model, nil
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		if msg := errorMessage(body); msg != "" {
			return Model{}, &ValidationError{Message: msg, Status: status}
		}
		return Model{}, fmt.Errorf("request failed: %d", status)
	case http.StatusUnauthorized:
		return Model{}, fmt.Errorf("request failed: %w", client.ErrUnauthorized)
	case http.StatusForbidden:
		return Model{}, fmt.Errorf("request failed: %w", client.ErrForbidden)
	case http.StatusNotFound:
		return Model{}, fmt.Errorf("request failed: %w", client.ErrNotFound)
	case http.StatusConflict:
		return Model{}, fmt.Errorf("request failed: %w", client.ErrConflict)
	default:
		return Model{}, fmt.Errorf("request failed: %d", status)
	}
}

// checkWriteStatus mappar skrivstatus utan body till klassificerbara fel
// (401/403 för toast/redirect via AccessDenied-middleware).
func checkWriteStatus(status int) error {
	switch status {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		return nil
	case http.StatusUnauthorized:
		return fmt.Errorf("request failed: %w", client.ErrUnauthorized)
	case http.StatusForbidden:
		return fmt.Errorf("request failed: %w", client.ErrForbidden)
	case http.StatusNotFound:
		return fmt.Errorf("request failed: %w", client.ErrNotFound)
	case http.StatusConflict:
		return fmt.Errorf("request failed: %w", client.ErrConflict)
	default:
		return fmt.Errorf("request failed: %d", status)
	}
}
