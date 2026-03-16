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

	"github.com/diwise/diwise-web/internal/presentation/api/authz"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var ErrNotFound = fmt.Errorf("not found")
var ErrUnauthorized = fmt.Errorf("unauthorized")
var ErrConflict = fmt.Errorf("conflict")

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
	httpClient           http.Client
}

func NewClient(devmgmt, things, admin, alarms, measurement string) *Client {
	return &Client{
		deviceManagementURL:  devmgmt,
		sensorsManagementURL: strings.Replace(devmgmt, "/device", "/sensor", 1),
		thingManagementURL:   things,
		adminURL:             admin,
		alarmsURL:            alarms,
		measurementURL:       measurement,
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

func (c *Client) Get(ctx context.Context, baseURL, path string, params url.Values) (*ApiResponse, error) {
	if strings.ContainsAny(path, "/") {
		path = strings.TrimPrefix(path, "/")
		path = strings.TrimSuffix(path, "/")
	}

	log := logging.GetFromContext(ctx).With(slog.String("url", baseURL), slog.String("path", path))
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
	req.Header.Add("Authorization", "Bearer "+authz.Token(ctx))
	req.Header.Add("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error("could not send get request", "error", err)
		return nil, fmt.Errorf("failed to send get request: %s", err.Error())
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("could not read response body", "error", err)
		return nil, fmt.Errorf("failed to read response body: %s", err.Error())
	}

	if resp.StatusCode == http.StatusUnauthorized {
		log.Error("request failed with unauthorized status")
		return nil, fmt.Errorf("request failed: %w", ErrUnauthorized)
	}
	if resp.StatusCode == http.StatusNotFound {
		log.Error("request failed with not found status")
		return nil, fmt.Errorf("request failed: %w", ErrNotFound)
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

func (c *Client) Patch(ctx context.Context, baseURL, id string, body []byte) error {
	log := logging.GetFromContext(ctx).With(slog.String("url", baseURL), slog.String("id", id))

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
	req.Header.Add("Authorization", "Bearer "+authz.Token(ctx))
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error("could not send patch request", "error", err)
		return fmt.Errorf("failed to send patch request: %s", err.Error())
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		log.Error("request failed with unauthorized status")
		return fmt.Errorf("request failed: %w", ErrUnauthorized)
	}
	if resp.StatusCode == http.StatusNotFound {
		log.Error("request failed with not found status")
		return fmt.Errorf("request failed: %w", ErrNotFound)
	}
	if resp.StatusCode == http.StatusConflict {
		log.Error("request failed with conflict status")
		return fmt.Errorf("request failed: %w", ErrConflict)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		log.Error("request failed with status code", "statusCode", resp.StatusCode)
		return fmt.Errorf("request failed: %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) Post(ctx context.Context, baseURL string, body []byte) error {
	log := logging.GetFromContext(ctx).With(slog.String("url", baseURL))

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
	req.Header.Add("Authorization", "Bearer "+authz.Token(ctx))
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error("could not send post request", "error", err)
		return fmt.Errorf("failed to send post request: %s", err.Error())
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		log.Error("request failed with unauthorized status")
		return fmt.Errorf("request failed: %w", ErrUnauthorized)
	}
	if resp.StatusCode == http.StatusNotFound {
		log.Error("request failed with not found status")
		return fmt.Errorf("request failed: %w", ErrNotFound)
	}
	if resp.StatusCode == http.StatusConflict {
		log.Error("request failed with conflict status")
		return fmt.Errorf("request failed: %w", ErrConflict)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		log.Error("request failed with status code", "statusCode", resp.StatusCode)
		return fmt.Errorf("request failed: %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) Put(ctx context.Context, baseURL string, body []byte) error {
	log := logging.GetFromContext(ctx).With(slog.String("url", baseURL))

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
	req.Header.Add("Authorization", "Bearer "+authz.Token(ctx))
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error("could not send put request", "error", err)
		return fmt.Errorf("failed to send put request: %s", err.Error())
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		log.Error("request failed with unauthorized status")
		return fmt.Errorf("request failed: %w", ErrUnauthorized)
	}
	if resp.StatusCode == http.StatusNotFound {
		log.Error("request failed with not found status")
		return fmt.Errorf("request failed: %w", ErrNotFound)
	}
	if resp.StatusCode == http.StatusConflict {
		log.Error("request failed with conflict status")
		return fmt.Errorf("request failed: %w", ErrConflict)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		log.Error("request failed with status code", "statusCode", resp.StatusCode)
		return fmt.Errorf("request failed: %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) Delete(ctx context.Context, baseURL string) error {
	log := logging.GetFromContext(ctx).With(slog.String("url", baseURL))

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
	req.Header.Add("Authorization", "Bearer "+authz.Token(ctx))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error("could not send delete request", "error", err)
		return fmt.Errorf("failed to send delete request: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		log.Error("request failed with unauthorized status")
		return fmt.Errorf("request failed: %w", ErrUnauthorized)
	}
	if resp.StatusCode == http.StatusNotFound {
		log.Error("request failed with not found status")
		return fmt.Errorf("request failed: %w", ErrNotFound)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		log.Error("request failed with status code", "statusCode", resp.StatusCode)
		return fmt.Errorf("request failed: %d", resp.StatusCode)
	}

	return nil
}
