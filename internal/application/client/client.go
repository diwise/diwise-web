package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("diwise-web/app/client")

var ErrNotFound = fmt.Errorf("not found")
var ErrUnauthorized = fmt.Errorf("unauthorized")
var ErrForbidden = fmt.Errorf("forbidden")
var ErrConflict = fmt.Errorf("conflict")
var ErrBadRequest = fmt.Errorf("bad request")
var ErrUnprocessable = fmt.Errorf("unprocessable entity")

// maxResponseBodyBytes bounds backend response bodies (same order as
// things-v2 maxRequestBodyBytes 1MiB, with headroom for list/export payloads).
const maxResponseBodyBytes = 4 << 20

// ValidatePathID rejects empty IDs and path separators before they become
// URL segments (thing/template/variant/model IDs must not contain "/").
// UUIDs are validated by callers with uuid.Parse where required.
func ValidatePathID(id, what string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: %s id is empty", ErrBadRequest, what)
	}
	if strings.Contains(id, "/") {
		return fmt.Errorf("%w: %s id %q must not contain %q", ErrBadRequest, what, id, "/")
	}
	return nil
}

// BackendError carries a classified backend failure with its text for form
// display (400/422 validation details, 409 conflicts). errors.Is matches
// the sentinel (ErrBadRequest/ErrUnprocessable/ErrNotFound/ErrConflict/...).
type BackendError struct {
	Status  int
	Message string
}

func (e *BackendError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("request failed: %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("request failed: %d", e.Status)
}

func (e *BackendError) Unwrap() error {
	switch e.Status {
	case http.StatusBadRequest:
		return ErrBadRequest
	case http.StatusUnprocessableEntity:
		return ErrUnprocessable
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusConflict:
		return ErrConflict
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	default:
		return nil
	}
}

// errorMessage extracts backend {error: string} text without failing on
// empty/non-JSON bodies.
func errorMessage(body []byte) string {
	var m struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &m); err != nil {
		return strings.TrimSpace(string(body))
	}
	return m.Error
}

// readLimitedBody reads at most maxResponseBodyBytes+1 and reports overflow
// as ErrBadRequest so callers never OOM on backend payloads.
func readLimitedBody(resp *http.Response) ([]byte, error) {
	limited := io.LimitReader(resp.Body, maxResponseBodyBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if len(raw) > maxResponseBodyBytes {
		return nil, fmt.Errorf("response body exceeds %d bytes: %w", maxResponseBodyBytes, ErrBadRequest)
	}
	return raw, nil
}

func errUnauthorized(ctx context.Context) error {
	MarkAuthDenied(ctx)
	return fmt.Errorf("request failed: %w", ErrUnauthorized)
}

func errForbidden(ctx context.Context) error {
	MarkPermissionDenied(ctx)
	return fmt.Errorf("request failed: %w", ErrForbidden)
}

func bearerToken(ctx context.Context) (string, error) {
	token := auth.Token(ctx)
	if token == "" {
		return "", errUnauthorized(ctx)
	}

	return token, nil
}

type Meta struct {
	TotalRecords uint64  `json:"totalRecords"`
	Offset       *uint64 `json:"offset,omitempty"`
	Limit        *uint64 `json:"limit,omitempty"`
	Count        uint64  `json:"count"`
}

type Links struct {
	Self  *string `json:"self,omitempty"`
	First *string `json:"first,omitempty"`
	Prev  *string `json:"prev,omitempty"`
	Next  *string `json:"next,omitempty"`
	Last  *string `json:"last,omitempty"`
}

type Resource struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type ApiResponse struct {
	Meta     *Meta           `json:"meta,omitempty"`
	Data     json.RawMessage `json:"data"`
	Links    *Links          `json:"links,omitempty"`
	Included []Resource      `json:"included,omitempty"`
}

type Client struct {
	deviceManagementURL  string
	sensorsManagementURL string
	thingManagementURL   string
	adminURL             string
	measurementURL       string
	alarmsURL            string
	transformURL         string
	httpClient           http.Client
}

func NewClient(devmgmt, things, admin, alarms, measurement, transformURL string) *Client {
	return &Client{
		deviceManagementURL:  devmgmt,
		sensorsManagementURL: strings.Replace(devmgmt, "/device", "/sensor", 1),
		thingManagementURL:   things,
		adminURL:             admin,
		alarmsURL:            alarms,
		measurementURL:       measurement,
		transformURL:         transformURL,
		httpClient: http.Client{
			Transport: otelhttp.NewTransport(&http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}),
			Timeout:   10 * time.Second,
		},
	}
}

func (c *Client) DeviceManagementURL() string  { return c.deviceManagementURL }
func (c *Client) SensorsManagementURL() string { return c.sensorsManagementURL }
func (c *Client) ThingManagementURL() string   { return c.thingManagementURL }
func (c *Client) AdminURL() string             { return c.adminURL }
func (c *Client) MeasurementURL() string       { return c.measurementURL }
func (c *Client) AlarmsURL() string            { return c.alarmsURL }
func (c *Client) TransformURL() string         { return c.transformURL }

func (c *Client) Get(ctx context.Context, baseURL, path string, params url.Values) (respOut *ApiResponse, err error) {
	ctx, span := tracer.Start(ctx, "client.get")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	if strings.ContainsAny(path, "/") {
		path = strings.TrimPrefix(path, "/")
		path = strings.TrimSuffix(path, "/")
	}

	log := logging.GetFromContext(ctx).With(slog.String("url", baseURL), slog.String("path", path))
	token, err := bearerToken(ctx)
	if err != nil {
		log.Error("missing auth token, refusing backend request")
		return nil, err
	}
	u, err := url.Parse(strings.TrimSuffix(fmt.Sprintf("%s/%s", baseURL, path), "/"))
	if err != nil {
		log.Error("could not parse url", "error", err)
		return nil, fmt.Errorf("could not parse url: %s", err.Error())
	}

	u.RawQuery = params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		log.Error("could not create http request", "error", err)
		return nil, fmt.Errorf("failed to create http request: %s", err.Error())
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error("could not send get request", "error", err)
		return nil, fmt.Errorf("failed to send get request: %s", err.Error())
	}
	defer resp.Body.Close()

	body, err := readLimitedBody(resp)
	if err != nil {
		log.Error("could not read response body", "error", err)
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		log.Error("request failed with unauthorized status")
		return nil, errUnauthorized(ctx)
	}
	if resp.StatusCode == http.StatusForbidden {
		log.Error("request failed with forbidden status")
		return nil, errForbidden(ctx)
	}
	if resp.StatusCode == http.StatusNotFound {
		log.Error("request failed with not found status")
		return nil, fmt.Errorf("request failed: %w", ErrNotFound)
	}
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity {
		log.Error("request failed with validation status", "statusCode", resp.StatusCode, "responseBody", string(body))
		return nil, &BackendError{Status: resp.StatusCode, Message: errorMessage(body)}
	}
	if resp.StatusCode >= http.StatusBadRequest {
		log.Error("request failed with status code", "statusCode", resp.StatusCode, "responseBody", string(body))
		return nil, fmt.Errorf("request failed: %d", resp.StatusCode)
	}

	if string(body) == "[]" {
		var arr json.RawMessage
		_ = json.Unmarshal(body, &arr)
		return &ApiResponse{Data: arr}, nil
	}

	impl := ApiResponse{}
	if err := json.Unmarshal(body, &impl); err != nil {
		log.Error("could not unmarshal response body", "error", err, "responseBody", string(body))
		return nil, fmt.Errorf("failed to unmarshal response body: %s", err.Error())
	}

	return &impl, nil
}

// GetRaw performs a GET against a non-envelope JSON API (iot-things-v2
// returns bare arrays with totals in headers, not {meta, data}).
// The body is returned raw with the response headers.
func (c *Client) GetRaw(ctx context.Context, baseURL, path string, params url.Values) (bodyOut []byte, headerOut http.Header, err error) {
	ctx, span := tracer.Start(ctx, "client.get-raw")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	if strings.ContainsAny(path, "/") {
		path = strings.TrimPrefix(path, "/")
		path = strings.TrimSuffix(path, "/")
	}

	log := logging.GetFromContext(ctx).With(slog.String("url", baseURL), slog.String("path", path))
	token, err := bearerToken(ctx)
	if err != nil {
		log.Error("missing auth token, refusing backend request")
		return nil, nil, err
	}
	u, err := url.Parse(strings.TrimSuffix(fmt.Sprintf("%s/%s", baseURL, path), "/"))
	if err != nil {
		log.Error("could not parse url", "error", err)
		return nil, nil, fmt.Errorf("could not parse url: %s", err.Error())
	}

	u.RawQuery = params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		log.Error("could not create http request", "error", err)
		return nil, nil, fmt.Errorf("failed to create http request: %s", err.Error())
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error("could not send get request", "error", err)
		return nil, nil, fmt.Errorf("failed to send get request: %s", err.Error())
	}
	defer resp.Body.Close()

	body, err := readLimitedBody(resp)
	if err != nil {
		log.Error("could not read response body", "error", err)
		return nil, nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		log.Error("request failed with unauthorized status")
		return nil, nil, errUnauthorized(ctx)
	}
	if resp.StatusCode == http.StatusForbidden {
		log.Error("request failed with forbidden status")
		return nil, nil, errForbidden(ctx)
	}
	if resp.StatusCode == http.StatusNotFound {
		log.Error("request failed with not found status")
		return nil, nil, fmt.Errorf("request failed: %w", ErrNotFound)
	}
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity {
		log.Error("request failed with validation status", "statusCode", resp.StatusCode, "responseBody", string(body))
		return nil, nil, &BackendError{Status: resp.StatusCode, Message: errorMessage(body)}
	}
	if resp.StatusCode >= http.StatusBadRequest {
		log.Error("request failed with status code", "statusCode", resp.StatusCode, "responseBody", string(body))
		return nil, nil, fmt.Errorf("request failed: %d", resp.StatusCode)
	}

	return body, resp.Header, nil
}

// WriteRaw sends POST/PUT/DELETE with query params and extra headers
// (e.g. If-Match), returning response body and header. 404/409 map to
// ErrNotFound/ErrConflict, 400/422 to BackendError with backend text,
// like the other helpers.
func (c *Client) WriteRaw(ctx context.Context, method, baseURL, path string, params url.Values, headers map[string]string, body []byte) (bodyOut []byte, headerOut http.Header, err error) {
	ctx, span := tracer.Start(ctx, "client.write-raw")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	if strings.ContainsAny(path, "/") {
		path = strings.TrimPrefix(path, "/")
		path = strings.TrimSuffix(path, "/")
	}

	log := logging.GetFromContext(ctx).With(slog.String("url", baseURL), slog.String("path", path))
	token, err := bearerToken(ctx)
	if err != nil {
		log.Error("missing auth token, refusing backend request")
		return nil, nil, err
	}
	u, err := url.Parse(strings.TrimSuffix(fmt.Sprintf("%s/%s", baseURL, path), "/"))
	if err != nil {
		log.Error("could not parse url", "error", err)
		return nil, nil, fmt.Errorf("could not parse url: %s", err.Error())
	}

	u.RawQuery = params.Encode()
	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		log.Error("could not create http request", "error", err)
		return nil, nil, fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error("could not send write request", "error", err)
		return nil, nil, fmt.Errorf("failed to send write request: %s", err.Error())
	}
	defer resp.Body.Close()

	responseBody, err := readLimitedBody(resp)
	if err != nil {
		log.Error("could not read response body", "error", err)
		return nil, nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		log.Error("request failed with unauthorized status")
		return nil, nil, errUnauthorized(ctx)
	}
	if resp.StatusCode == http.StatusForbidden {
		log.Error("request failed with forbidden status")
		return nil, nil, errForbidden(ctx)
	}
	if resp.StatusCode == http.StatusNotFound {
		log.Error("request failed with not found status")
		return nil, nil, fmt.Errorf("request failed: %w", ErrNotFound)
	}
	if resp.StatusCode == http.StatusConflict {
		log.Error("request failed with conflict status")
		return nil, nil, fmt.Errorf("request failed: %w", ErrConflict)
	}
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity {
		log.Error("request failed with validation status", "statusCode", resp.StatusCode, "responseBody", string(responseBody))
		return nil, nil, &BackendError{Status: resp.StatusCode, Message: errorMessage(responseBody)}
	}
	if resp.StatusCode >= http.StatusBadRequest {
		log.Error("request failed with status code", "statusCode", resp.StatusCode, "responseBody", string(responseBody))
		return nil, nil, fmt.Errorf("request failed: %d", resp.StatusCode)
	}

	return responseBody, resp.Header, nil
}

// WriteRawDetailed sends POST/PUT/DELETE like WriteRaw but returns the HTTP
// status and body for all responses (only transport errors surface as err),
// so callers can surface backend error texts (e.g. rule validation
// {error}) in forms. 401/403 still mark access-denied for toasts.
func (c *Client) WriteRawDetailed(ctx context.Context, method, baseURL, path string, params url.Values, headers map[string]string, body []byte) (bodyOut []byte, headerOut http.Header, statusOut int, err error) {
	ctx, span := tracer.Start(ctx, "client.write-raw-detailed")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	if strings.ContainsAny(path, "/") {
		path = strings.TrimPrefix(path, "/")
		path = strings.TrimSuffix(path, "/")
	}

	log := logging.GetFromContext(ctx).With(slog.String("url", baseURL), slog.String("path", path))
	token, err := bearerToken(ctx)
	if err != nil {
		log.Error("missing auth token, refusing backend request")
		return nil, nil, 0, err
	}
	u, err := url.Parse(strings.TrimSuffix(fmt.Sprintf("%s/%s", baseURL, path), "/"))
	if err != nil {
		log.Error("could not parse url", "error", err)
		return nil, nil, 0, fmt.Errorf("could not parse url: %s", err.Error())
	}

	u.RawQuery = params.Encode()
	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		log.Error("could not create http request", "error", err)
		return nil, nil, 0, fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error("could not send write request", "error", err)
		return nil, nil, 0, fmt.Errorf("failed to send write request: %s", err.Error())
	}
	defer resp.Body.Close()

	responseBody, err := readLimitedBody(resp)
	if err != nil {
		log.Error("could not read response body", "error", err)
		return nil, nil, 0, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		log.Error("request failed with unauthorized status")
		MarkAuthDenied(ctx)
	}
	if resp.StatusCode == http.StatusForbidden {
		log.Error("request failed with forbidden status")
		MarkPermissionDenied(ctx)
	}

	return responseBody, resp.Header, resp.StatusCode, nil
}

func (c *Client) Patch(ctx context.Context, baseURL, id string, body []byte) (err error) {
	ctx, span := tracer.Start(ctx, "client.patch")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	log := logging.GetFromContext(ctx).With(slog.String("url", baseURL), slog.String("id", id))

	token, err := bearerToken(ctx)
	if err != nil {
		log.Error("missing auth token, refusing backend request")
		return err
	}

	u, err := url.Parse(strings.TrimSuffix(fmt.Sprintf("%s/%s", baseURL, id), "/"))
	if err != nil {
		log.Error("could not parse url", "error", err)
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, u.String(), bytes.NewReader(body))
	if err != nil {
		log.Error("could not create http request", "error", err)
		return fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error("could not send patch request", "error", err)
		return fmt.Errorf("failed to send patch request: %s", err.Error())
	}
	defer resp.Body.Close()
	respBody, _ := readLimitedBody(resp)

	if resp.StatusCode == http.StatusUnauthorized {
		log.Error("request failed with unauthorized status")
		return errUnauthorized(ctx)
	}
	if resp.StatusCode == http.StatusForbidden {
		log.Error("request failed with forbidden status")
		return errForbidden(ctx)
	}
	if resp.StatusCode == http.StatusNotFound {
		log.Error("request failed with not found status")
		return fmt.Errorf("request failed: %w", ErrNotFound)
	}
	if resp.StatusCode == http.StatusConflict {
		log.Error("request failed with conflict status")
		return fmt.Errorf("request failed: %w", ErrConflict)
	}
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity {
		return &BackendError{Status: resp.StatusCode, Message: errorMessage(respBody)}
	}
	if resp.StatusCode >= http.StatusBadRequest {
		log.Error("request failed with status code", "statusCode", resp.StatusCode)
		return fmt.Errorf("request failed: %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) Post(ctx context.Context, baseURL string, body []byte) (err error) {
	ctx, span := tracer.Start(ctx, "client.post")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	log := logging.GetFromContext(ctx).With(slog.String("url", baseURL))

	token, err := bearerToken(ctx)
	if err != nil {
		log.Error("missing auth token, refusing backend request")
		return err
	}

	u, err := url.Parse(strings.TrimSuffix(baseURL, "/"))
	if err != nil {
		log.Error("could not parse url", "error", err)
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		log.Error("could not create http request", "error", err)
		return fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error("could not send post request", "error", err)
		return fmt.Errorf("failed to send post request: %s", err.Error())
	}
	defer resp.Body.Close()
	respBody, _ := readLimitedBody(resp)

	if resp.StatusCode == http.StatusUnauthorized {
		log.Error("request failed with unauthorized status")
		return errUnauthorized(ctx)
	}
	if resp.StatusCode == http.StatusForbidden {
		log.Error("request failed with forbidden status")
		return errForbidden(ctx)
	}
	if resp.StatusCode == http.StatusNotFound {
		log.Error("request failed with not found status")
		return fmt.Errorf("request failed: %w", ErrNotFound)
	}
	if resp.StatusCode == http.StatusConflict {
		log.Error("request failed with conflict status")
		return fmt.Errorf("request failed: %w", ErrConflict)
	}
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity {
		return &BackendError{Status: resp.StatusCode, Message: errorMessage(respBody)}
	}
	if resp.StatusCode >= http.StatusBadRequest {
		log.Error("request failed with status code", "statusCode", resp.StatusCode)
		return fmt.Errorf("request failed: %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) Put(ctx context.Context, baseURL string, body []byte) (err error) {
	ctx, span := tracer.Start(ctx, "client.put")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	log := logging.GetFromContext(ctx).With(slog.String("url", baseURL))

	token, err := bearerToken(ctx)
	if err != nil {
		log.Error("missing auth token, refusing backend request")
		return err
	}

	u, err := url.Parse(strings.TrimSuffix(baseURL, "/"))
	if err != nil {
		log.Error("could not parse url", "error", err)
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), bytes.NewReader(body))
	if err != nil {
		log.Error("could not create http request", "error", err)
		return fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error("could not send put request", "error", err)
		return fmt.Errorf("failed to send put request: %s", err.Error())
	}
	defer resp.Body.Close()
	respBody, _ := readLimitedBody(resp)

	if resp.StatusCode == http.StatusUnauthorized {
		log.Error("request failed with unauthorized status")
		return errUnauthorized(ctx)
	}
	if resp.StatusCode == http.StatusForbidden {
		log.Error("request failed with forbidden status")
		return errForbidden(ctx)
	}
	if resp.StatusCode == http.StatusNotFound {
		log.Error("request failed with not found status")
		return fmt.Errorf("request failed: %w", ErrNotFound)
	}
	if resp.StatusCode == http.StatusConflict {
		log.Error("request failed with conflict status")
		return fmt.Errorf("request failed: %w", ErrConflict)
	}
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity {
		return &BackendError{Status: resp.StatusCode, Message: errorMessage(respBody)}
	}
	if resp.StatusCode >= http.StatusBadRequest {
		log.Error("request failed with status code", "statusCode", resp.StatusCode)
		return fmt.Errorf("request failed: %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) Delete(ctx context.Context, baseURL string) (err error) {
	ctx, span := tracer.Start(ctx, "client.delete")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	log := logging.GetFromContext(ctx).With(slog.String("url", baseURL))

	token, err := bearerToken(ctx)
	if err != nil {
		log.Error("missing auth token, refusing backend request")
		return err
	}

	u, err := url.Parse(strings.TrimSuffix(baseURL, "/"))
	if err != nil {
		log.Error("could not parse url", "error", err)
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u.String(), nil)
	if err != nil {
		log.Error("could not create http request", "error", err)
		return fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error("could not send delete request", "error", err)
		return fmt.Errorf("failed to send delete request: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := readLimitedBody(resp)

	if resp.StatusCode == http.StatusUnauthorized {
		log.Error("request failed with unauthorized status")
		return errUnauthorized(ctx)
	}
	if resp.StatusCode == http.StatusForbidden {
		log.Error("request failed with forbidden status")
		return errForbidden(ctx)
	}
	if resp.StatusCode == http.StatusNotFound {
		log.Error("request failed with not found status")
		return fmt.Errorf("request failed: %w", ErrNotFound)
	}
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity {
		return &BackendError{Status: resp.StatusCode, Message: errorMessage(respBody)}
	}
	if resp.StatusCode >= http.StatusBadRequest {
		log.Error("request failed with status code", "statusCode", resp.StatusCode)
		return fmt.Errorf("request failed: %d", resp.StatusCode)
	}

	return nil
}
