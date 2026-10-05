package thingsv2

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
	if f.VariantVersion != "" {
		params.Add("variantVersion", f.VariantVersion)
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
	for k, v := range f.Metadata {
		if k != "" && v != "" {
			params.Add("meta."+k, v)
		}
	}
	if f.BBox != nil {
		params.Add("bbox", f.BBox.String())
	}
	if f.Near != nil {
		params.Add("near", f.Near.String())
	}
	if f.Limit > 0 {
		params.Add("limit", strconv.Itoa(f.Limit))
	}
	if f.Offset > 0 {
		params.Add("offset", strconv.Itoa(f.Offset))
	}
	return params
}

// Configured reports whether a backend URL is set (empty THINGS_V2_URL
// means pages render "not configured" instead of calling out).
func (s *Service) Configured() bool {
	return s.baseURL != ""
}

// decodeSingleJSON decodes exactly one JSON document (no trailing data).
// encoding/json Unmarshal already rejects trailing data after the top-level
// value, so this is a thin wrapper that adds context.
func decodeSingleJSON(body []byte, dst any, what string) error {
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("failed to decode %s: %w", what, err)
	}
	return nil
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
	if err := decodeSingleJSON(body, &things, "things"); err != nil {
		return Result{}, err
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

	if err = client.ValidatePathID(id, "thing"); err != nil {
		return Thing{}, err
	}

	params := url.Values{}
	params.Add("tenant", tenant)

	body, _, err := s.client.GetRaw(ctx, s.baseURL, "things/"+id, params)
	if err != nil {
		return Thing{}, err
	}

	var thing Thing
	if err = decodeSingleJSON(body, &thing, "thing"); err != nil {
		return Thing{}, err
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
	if err = decodeSingleJSON(body, &specs, "templates"); err != nil {
		return nil, err
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
	if err = decodeSingleJSON(body, &tags, "tags"); err != nil {
		return nil, err
	}
	if tags == nil {
		tags = []string{}
	}

	return tags, nil
}

// GetHistory reads property history (oldest first) in [from, to].
// Empty property means all properties; empty from/to means unbounded.
// HistoryPoint carries receivedAt/source/messageId/late so GUI can show
// current vs latest delta vs invalid observation.
func (s *Service) GetHistory(ctx context.Context, tenant, id, property string, from, to time.Time, limit int) ([]HistoryPoint, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-history-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	if err = client.ValidatePathID(id, "thing"); err != nil {
		return nil, err
	}

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
	if err = decodeSingleJSON(body, &points, "history"); err != nil {
		return nil, err
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

	if err = client.ValidatePathID(id, "thing"); err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Add("tenant", tenant)

	body, _, err := s.client.GetRaw(ctx, s.baseURL, "things/"+id+"/bindings", params)
	if err != nil {
		return nil, err
	}

	var response struct {
		Bindings []Binding `json:"bindings"`
	}
	if err = decodeSingleJSON(body, &response, "bindings"); err != nil {
		return nil, err
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

	if err = client.ValidatePathID(id, "thing"); err != nil {
		return Overview{}, err
	}

	params := url.Values{}
	params.Add("tenant", tenant)

	body, _, err := s.client.GetRaw(ctx, s.baseURL, "things/"+id+"/overview", params)
	if err != nil {
		return Overview{}, err
	}

	var overview Overview
	if err = decodeSingleJSON(body, &overview, "overview"); err != nil {
		return Overview{}, err
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
	if err = decodeSingleJSON(body, &specs, "variants"); err != nil {
		return nil, err
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

	if err = client.ValidatePathID(id, "thing"); err != nil {
		return EffectiveConfig{}, err
	}

	params := url.Values{}
	params.Add("tenant", tenant)

	body, _, err := s.client.GetRaw(ctx, s.baseURL, "things/"+id+"/config", params)
	if err != nil {
		return EffectiveConfig{}, err
	}

	var config EffectiveConfig
	if err = decodeSingleJSON(body, &config, "config"); err != nil {
		return EffectiveConfig{}, err
	}

	return config, nil
}

// GetRelations reads a thing's named relations with resolved targets
// (generic slots; partOf is one slot). Missing targets decode as nil Target
// so broken references stay visible. Bootstrap/diagnostics companion to
// ExportThings.
func (s *Service) GetRelations(ctx context.Context, tenant, id string) ([]RelationResolve, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-relations-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	if err = client.ValidatePathID(id, "thing"); err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Add("tenant", tenant)

	body, _, err := s.client.GetRaw(ctx, s.baseURL, "things/"+id+"/relations", params)
	if err != nil {
		return nil, err
	}

	var response struct {
		Relations []RelationResolve `json:"relations"`
	}
	if err = decodeSingleJSON(body, &response, "relations"); err != nil {
		return nil, err
	}
	if response.Relations == nil {
		response.Relations = []RelationResolve{}
	}

	return response.Relations, nil
}

// ExportThings returns all things as recreatable ObjectSpecs (bootstrap/
// migration underlay; same shape as POST /things). Sorted by tenant+ID
// server-side; empty is [] never null.
func (s *Service) ExportThings(ctx context.Context, tenant string) ([]ObjectSpec, error) {
	var err error
	ctx, span := tracer.Start(ctx, "export-things-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	if tenant != "" {
		params.Add("tenant", tenant)
	}

	body, _, err := s.client.GetRaw(ctx, s.baseURL, "things/export", params)
	if err != nil {
		return nil, err
	}

	var specs []ObjectSpec
	if err = decodeSingleJSON(body, &specs, "export"); err != nil {
		return nil, err
	}
	if specs == nil {
		specs = []ObjectSpec{}
	}

	return specs, nil
}

// GetTree reads a subtree rooted at id (depth >= 0, backend caps at 10).
func (s *Service) GetTree(ctx context.Context, tenant, id string, depth int) (TreeNode, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-tree-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	if err = client.ValidatePathID(id, "thing"); err != nil {
		return TreeNode{}, err
	}
	if depth < 0 {
		return TreeNode{}, fmt.Errorf("invalid depth %d: %w", depth, client.ErrBadRequest)
	}

	params := url.Values{}
	params.Add("tenant", tenant)
	if depth > 0 {
		params.Add("depth", strconv.Itoa(depth))
	}

	body, _, err := s.client.GetRaw(ctx, s.baseURL, "things/"+id+"/tree", params)
	if err != nil {
		return TreeNode{}, err
	}

	var node TreeNode
	if err = decodeSingleJSON(body, &node, "tree"); err != nil {
		return TreeNode{}, err
	}

	return node, nil
}

// PreviewThing compiles without storing: effective config with param origins
// before create/update. No If-Match (safe+idempotent).
func (s *Service) PreviewThing(ctx context.Context, tenant, id string, spec ObjectSpec) (EffectiveConfig, error) {
	var err error
	ctx, span := tracer.Start(ctx, "preview-thing-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	if err = client.ValidatePathID(id, "thing"); err != nil {
		return EffectiveConfig{}, err
	}
	spec.Tenant = tenant
	spec.ThingID = id

	params := url.Values{}
	params.Add("tenant", tenant)

	raw, err := json.Marshal(spec)
	if err != nil {
		return EffectiveConfig{}, fmt.Errorf("failed to encode thing: %w", err)
	}

	body, _, err := s.client.WriteRaw(ctx, http.MethodPut, s.baseURL, "things/"+id+"/preview", params, nil, raw)
	if err != nil {
		return EffectiveConfig{}, err
	}

	var config EffectiveConfig
	if err = decodeSingleJSON(body, &config, "preview"); err != nil {
		return EffectiveConfig{}, err
	}

	return config, nil
}

// PreviewUpgrade diffs pinned config against new template/variant versions
// without changing anything. Overrides dropped by new versions surface as
// conflicts (never vanish silently).
func (s *Service) PreviewUpgrade(ctx context.Context, tenant, id, newTemplateID, newTemplateVersion, newVariantID, newVariantVersion string) (UpgradeResult, error) {
	var err error
	ctx, span := tracer.Start(ctx, "preview-upgrade-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	if err = client.ValidatePathID(id, "thing"); err != nil {
		return UpgradeResult{}, err
	}
	if err = client.ValidatePathID(newTemplateID, "template"); err != nil {
		return UpgradeResult{}, err
	}
	if strings.TrimSpace(newTemplateVersion) == "" {
		return UpgradeResult{}, fmt.Errorf("template version is empty: %w", client.ErrBadRequest)
	}

	params := url.Values{}
	params.Add("tenant", tenant)

	raw, err := json.Marshal(map[string]string{
		"templateId":      newTemplateID,
		"templateVersion": newTemplateVersion,
		"variantId":       newVariantID,
		"variantVersion":  newVariantVersion,
	})
	if err != nil {
		return UpgradeResult{}, fmt.Errorf("failed to encode upgrade: %w", err)
	}

	body, _, err := s.client.WriteRaw(ctx, http.MethodPut, s.baseURL, "things/"+id+"/upgrade/preview", params, nil, raw)
	if err != nil {
		return UpgradeResult{}, err
	}

	var out UpgradeResult
	if err = decodeSingleJSON(body, &out, "upgrade preview"); err != nil {
		return UpgradeResult{}, err
	}
	if out.Conflicts == nil {
		out.Conflicts = []string{}
	}

	return out, nil
}

// UpgradeThing activates new versions. With conflicts and without force the
// backend denies (409 with diff+conflicts); force strips conflicting own
// choices. Revision is the config CAS stake.
func (s *Service) UpgradeThing(ctx context.Context, tenant, id, newTemplateID, newTemplateVersion, newVariantID, newVariantVersion string, force bool, revision int64) (UpgradeResult, error) {
	var err error
	ctx, span := tracer.Start(ctx, "upgrade-thing-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	if err = client.ValidatePathID(id, "thing"); err != nil {
		return UpgradeResult{}, err
	}
	if err = client.ValidatePathID(newTemplateID, "template"); err != nil {
		return UpgradeResult{}, err
	}
	if strings.TrimSpace(newTemplateVersion) == "" {
		return UpgradeResult{}, fmt.Errorf("template version is empty: %w", client.ErrBadRequest)
	}

	params := url.Values{}
	params.Add("tenant", tenant)
	if force {
		params.Add("force", "true")
	}

	raw, err := json.Marshal(map[string]string{
		"templateId":      newTemplateID,
		"templateVersion": newTemplateVersion,
		"variantId":       newVariantID,
		"variantVersion":  newVariantVersion,
	})
	if err != nil {
		return UpgradeResult{}, fmt.Errorf("failed to encode upgrade: %w", err)
	}

	body, _, status, err := s.client.WriteRawDetailed(ctx, http.MethodPut, s.baseURL, "things/"+id+"/upgrade", params,
		map[string]string{"If-Match": fmt.Sprintf(`"rev-%d"`, revision)}, raw)
	if err != nil {
		return UpgradeResult{}, err
	}
	// 409 with diff+conflicts is a classified conflict carrying underlay for
	// GUI: decode when possible, still return ErrConflict.
	if status == http.StatusConflict {
		var out UpgradeResult
		if decErr := json.Unmarshal(body, &out); decErr == nil {
			if out.Conflicts == nil {
				out.Conflicts = []string{}
			}
			return out, fmt.Errorf("upgrade conflicts: %w", client.ErrConflict)
		}
		return UpgradeResult{}, fmt.Errorf("request failed: %w", client.ErrConflict)
	}
	if status == http.StatusBadRequest || status == http.StatusUnprocessableEntity {
		msg := ""
		var m struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &m) == nil {
			msg = m.Error
		}
		if msg != "" {
			return UpgradeResult{}, &client.BackendError{Status: status, Message: msg}
		}
		return UpgradeResult{}, fmt.Errorf("request failed: %d", status)
	}
	if status == http.StatusUnauthorized {
		return UpgradeResult{}, fmt.Errorf("request failed: %w", client.ErrUnauthorized)
	}
	if status == http.StatusForbidden {
		return UpgradeResult{}, fmt.Errorf("request failed: %w", client.ErrForbidden)
	}
	if status == http.StatusNotFound {
		return UpgradeResult{}, fmt.Errorf("request failed: %w", client.ErrNotFound)
	}
	if status != http.StatusOK {
		return UpgradeResult{}, fmt.Errorf("request failed: %d", status)
	}

	var out UpgradeResult
	if err = decodeSingleJSON(body, &out, "upgrade"); err != nil {
		return UpgradeResult{}, err
	}
	if out.Conflicts == nil {
		out.Conflicts = []string{}
	}

	return out, nil
}

// CreateThing creates a thing from an object spec. Tenant in body never
// widens token auth: query ?tenant= wins (backend overwrites body tenant).
func (s *Service) CreateThing(ctx context.Context, tenant string, spec ObjectSpec) (Thing, error) {
	var err error
	ctx, span := tracer.Start(ctx, "create-thing-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	if err = client.ValidatePathID(spec.ThingID, "thing"); err != nil {
		return Thing{}, err
	}
	spec.Tenant = tenant

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
	if err = decodeSingleJSON(body, &thing, "thing"); err != nil {
		return Thing{}, err
	}

	return thing, nil
}

// UpdateThing replaces a thing from an object spec (revision is the
// config CAS stake; conflicts surface as client.ErrConflict).
func (s *Service) UpdateThing(ctx context.Context, tenant, id string, spec ObjectSpec, revision int64) (Thing, error) {
	var err error
	ctx, span := tracer.Start(ctx, "update-thing-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	if err = client.ValidatePathID(id, "thing"); err != nil {
		return Thing{}, err
	}
	spec.Tenant = tenant
	spec.ThingID = id

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
	if err = decodeSingleJSON(body, &thing, "thing"); err != nil {
		return Thing{}, err
	}

	return thing, nil
}

// DeleteThing deletes a thing (blocked with active children: ErrConflict).
func (s *Service) DeleteThing(ctx context.Context, tenant, id string) error {
	var err error
	ctx, span := tracer.Start(ctx, "delete-thing-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	if err = client.ValidatePathID(id, "thing"); err != nil {
		return err
	}

	params := url.Values{}
	params.Add("tenant", tenant)

	_, _, err = s.client.WriteRaw(ctx, http.MethodDelete, s.baseURL, "things/"+id, params, nil, nil)
	return err
}

// MoveParent switches a thing's generic relation parent atomically
// (currently partOf hierarchy; other slots via UpdateThing). Revision is the
// config CAS stake; conflicts surface as client.ErrConflict.
func (s *Service) MoveParent(ctx context.Context, tenant, id, parentID string, revision int64) (Thing, error) {
	var err error
	ctx, span := tracer.Start(ctx, "move-parent-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	if err = client.ValidatePathID(id, "thing"); err != nil {
		return Thing{}, err
	}
	if err = client.ValidatePathID(parentID, "parent thing"); err != nil {
		return Thing{}, err
	}

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
	if err = decodeSingleJSON(body, &thing, "thing"); err != nil {
		return Thing{}, err
	}

	return thing, nil
}

// UnlinkParent detaches a thing's partOf parent without deleting the thing.
func (s *Service) UnlinkParent(ctx context.Context, tenant, id string, revision int64) (Thing, error) {
	var err error
	ctx, span := tracer.Start(ctx, "unlink-parent-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	if err = client.ValidatePathID(id, "thing"); err != nil {
		return Thing{}, err
	}

	params := url.Values{}
	params.Add("tenant", tenant)

	body, _, err := s.client.WriteRaw(ctx, http.MethodDelete, s.baseURL, "things/"+id+"/parent", params,
		map[string]string{"If-Match": fmt.Sprintf(`"rev-%d"`, revision)}, nil)
	if err != nil {
		return Thing{}, err
	}

	var thing Thing
	if err = decodeSingleJSON(body, &thing, "thing"); err != nil {
		return Thing{}, err
	}

	return thing, nil
}

// SwapBindings replaces a thing's sensor bindings atomically (revision is
// the config CAS stake; conflicts surface as client.ErrConflict).
func (s *Service) SwapBindings(ctx context.Context, tenant, id string, bindings []Binding, revision int64) (Thing, error) {
	var err error
	ctx, span := tracer.Start(ctx, "swap-bindings-v2")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	if err = client.ValidatePathID(id, "thing"); err != nil {
		return Thing{}, err
	}

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
	if err = decodeSingleJSON(body, &thing, "thing"); err != nil {
		return Thing{}, err
	}

	return thing, nil
}
