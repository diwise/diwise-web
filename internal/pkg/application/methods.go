package application

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

	"github.com/diwise/diwise-web/internal/pkg/presentation/api/authz"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var ErrNotFound = fmt.Errorf("not found")
var ErrUnauthorized = fmt.Errorf("unauthorized")

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

var httpClient = http.Client{
	Transport: otelhttp.NewTransport(&http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}),
	Timeout: 10 * time.Second,
}

func (a *App) get(ctx context.Context, baseUrl, path string, params url.Values) (*ApiResponse, error) {
	if strings.ContainsAny(path, "/") {
		path = strings.TrimPrefix(path, "/")
		path = strings.TrimSuffix(path, "/")
	}

	log := logging.GetFromContext(ctx)

	u, err := url.Parse(strings.TrimSuffix(fmt.Sprintf("%s/%s", baseUrl, path), "/"))
	if err != nil {
		return nil, fmt.Errorf("could not parse url: %s", err.Error())
	}

	u.RawQuery = params.Encode()
	token := authz.Token(ctx)
	urlToGet := u.String()

	log = log.With("url", urlToGet)
	log.Debug("GET")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlToGet, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %s", err.Error())
	}

	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send get request: %s", err.Error())
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %s", err.Error())
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("request failed: %w", ErrUnauthorized)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("request failed: %w", ErrNotFound)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("request failed: %d", resp.StatusCode)
	}

	if string(respBody) == "[]" {
		var arr json.RawMessage
		json.Unmarshal(respBody, &arr)
		return &ApiResponse{
			Meta:  nil,
			Data:  arr,
			Links: nil,
		}, nil
	}

	impl := ApiResponse{}

	err = json.Unmarshal(respBody, &impl)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %s", err.Error())
	}

	log.Debug("body", slog.Any("data", impl.Data))

	return &impl, nil
}

func (a *App) patch(ctx context.Context, baseUrl, id string, body []byte) error {
	log := logging.GetFromContext(ctx)

	u, err := url.Parse(strings.TrimSuffix(fmt.Sprintf("%s/%s", baseUrl, id), "/"))
	if err != nil {
		return err
	}

	log = log.With("url", u.String())
	log.Debug("PATCH", slog.String("body", string(body)))

	token := authz.Token(ctx)

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, u.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send patch request: %s", err.Error())
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("request failed: %w", ErrUnauthorized)
	}

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("request failed: %w", ErrNotFound)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("request failed: %d", resp.StatusCode)
	}

	return nil
}

func (a *App) post(ctx context.Context, baseUrl string, body []byte) error {
	log := logging.GetFromContext(ctx)

	u, err := url.Parse(strings.TrimSuffix((baseUrl), "/"))
	if err != nil {
		return err
	}

	log = log.With("url", u.String())
	log.Debug("POST", slog.String("body", string(body)))

	token := authz.Token(ctx)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send post request: %s", err.Error())
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("request failed: %w", ErrUnauthorized)
	}

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("request failed: %w", ErrNotFound)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("request failed: %d", resp.StatusCode)
	}

	return nil
}

func (a *App) delete(ctx context.Context, baseUrl string) error {
	log := logging.GetFromContext(ctx)

	u, err := url.Parse(strings.TrimSuffix((baseUrl), "/"))
	if err != nil {
		return err
	}

	log = log.With("url", u.String())
	log.Debug("DELETE")

	token := authz.Token(ctx)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("request failed: %w", ErrUnauthorized)
	}

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("request failed: %w", ErrNotFound)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("request failed: %d", resp.StatusCode)
	}

	return nil
}
