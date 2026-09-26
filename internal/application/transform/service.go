package transform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("diwise-web/app/transform")

// ValidationError carries a backend 400 text (rule validation) for form
// display. The backend responds {error: string} in cleartext.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return "invalid rule: " + e.Message }

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

	params := url.Values{}
	params.Add("tenant", tenant)

	_, _, status, err := s.client.WriteRawDetailed(ctx, http.MethodDelete, s.baseURL, "models/"+id, params,
		map[string]string{"If-Match": fmt.Sprintf(`"rev-%d"`, revision)}, nil)
	if err != nil {
		return err
	}
	switch status {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("request failed: %w", client.ErrNotFound)
	case http.StatusConflict:
		return fmt.Errorf("request failed: %w", client.ErrConflict)
	default:
		return fmt.Errorf("request failed: %d", status)
	}
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
	if msg := errorMessage(body); msg != "" {
		return &ValidationError{Message: msg}
	}
	return fmt.Errorf("request failed: %d", status)
}

// PreviewRule dry-runs an inline rule against an example event.
func (s *Service) PreviewRule(ctx context.Context, tenant string, rule Rule, event PreviewEvent) (PreviewResult, error) {
	var err error
	ctx, span := tracer.Start(ctx, "preview-rule")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

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
		if msg := errorMessage(body); msg != "" {
			return PreviewResult{}, &ValidationError{Message: msg}
		}
		return PreviewResult{}, fmt.Errorf("request failed: %d", status)
	}

	var res PreviewResult
	if err = json.Unmarshal(body, &res); err != nil {
		return PreviewResult{}, fmt.Errorf("failed to decode preview: %w", err)
	}
	if res.Entities == nil {
		res.Entities = []PreviewEntity{}
	}

	return res, nil
}

func decodeModelResponse(body []byte, status int) (Model, error) {
	switch status {
	case http.StatusOK, http.StatusCreated:
		var model Model
		if err := json.Unmarshal(body, &model); err != nil {
			return Model{}, fmt.Errorf("failed to decode model: %w", err)
		}
		return model, nil
	case http.StatusBadRequest:
		if msg := errorMessage(body); msg != "" {
			return Model{}, &ValidationError{Message: msg}
		}
		return Model{}, fmt.Errorf("request failed: %d", status)
	case http.StatusNotFound:
		return Model{}, fmt.Errorf("request failed: %w", client.ErrNotFound)
	case http.StatusConflict:
		return Model{}, fmt.Errorf("request failed: %w", client.ErrConflict)
	default:
		return Model{}, fmt.Errorf("request failed: %d", status)
	}
}
