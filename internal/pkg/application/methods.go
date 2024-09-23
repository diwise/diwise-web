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
	"slices"
	"strings"

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

func (a *App) get(ctx context.Context, baseUrl, path string, params url.Values) (*ApiResponse, error) {
	if strings.ContainsAny(path, "/") {
		path = strings.TrimPrefix(path, "/")
		path = strings.TrimSuffix(path, "/")
	}

	log := logging.GetFromContext(ctx)

	u, err := url.Parse(strings.TrimSuffix(fmt.Sprintf("%s/%s", baseUrl, path), "/"))
	if err != nil {
		log.Error("could not parse url", "err", err.Error())
		return nil, err
	}

	u.RawQuery = params.Encode()
	token := authz.Token(ctx)
	urlToGet := u.String()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlToGet, nil)
	if err != nil {
		log.Error("failed to create http request", slog.String("url", urlToGet), "err", err.Error())
		err = fmt.Errorf("failed to create http request: %w", err)
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+token)

	transport := http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := http.Client{
		Transport: otelhttp.NewTransport(&transport),
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Error("http request failed", slog.String("url", urlToGet), "err", err.Error())
		err = fmt.Errorf("failed to retrieve information: %w", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		log.Error("unauthorized", slog.String("url", urlToGet))
		err = ErrUnauthorized
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		log.Error("not found", slog.String("url", urlToGet))
		err = ErrNotFound
		return nil, err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		log.Error("request failed", slog.String("url", urlToGet), slog.Int("status_code", resp.StatusCode))
		err = fmt.Errorf("request failed with status code %d", resp.StatusCode)
		return nil, err
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("failed to read response body: %w", err)
		return nil, err
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
		err = fmt.Errorf("failed to unmarshal response body: %w", err)
		return nil, err
	}

	log.Debug("body", slog.Any("data", impl.Data))

	return &impl, nil
}

func (a *App) patch(ctx context.Context, baseUrl, sensorID string, body []byte) error {
	log := logging.GetFromContext(ctx)

	u, err := url.Parse(strings.TrimSuffix(fmt.Sprintf("%s/%s", baseUrl, sensorID), "/"))
	if err != nil {
		return err
	}

	log.Debug("PATCH", slog.String("body", string(body)), slog.String("url", u.String()))

	token := authz.Token(ctx)

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, u.String(), bytes.NewReader(body))
	if err != nil {
		err = fmt.Errorf("failed to create http request: %w", err)
		return err
	}

	req.Header.Add("Authorization", "Bearer "+token)

	transport := http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := http.Client{
		Transport: otelhttp.NewTransport(&transport),
	}

	resp, err := client.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to patch: %w", err)
		return err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		err = fmt.Errorf("request failed, not authorized")
		return err
	}

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("request failed with status code %d", resp.StatusCode)
		return err
	}

	return nil
}

func (a *App) post(ctx context.Context, baseUrl string, body []byte) error {
	log := logging.GetFromContext(ctx)

	u, err := url.Parse(strings.TrimSuffix((baseUrl), "/"))
	if err != nil {
		return err
	}

	log.Debug("POST", slog.String("body", string(body)), slog.String("url", u.String()))

	token := authz.Token(ctx)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		err = fmt.Errorf("failed to create http request: %w", err)
		return err
	}

	req.Header.Add("Authorization", "Bearer "+token)

	transport := http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := http.Client{
		Transport: otelhttp.NewTransport(&transport),
	}

	resp, err := client.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to post: %w", err)
		return err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		err = fmt.Errorf("request failed, not authorized")
		return err
	}

	if !slices.Contains([]int{http.StatusCreated, http.StatusOK}, resp.StatusCode) {
		err = fmt.Errorf("request failed with status code %d", resp.StatusCode)
		return err
	}

	return nil
}

func (a *App) delete(ctx context.Context, baseUrl string) error {
	log := logging.GetFromContext(ctx)

	u, err := url.Parse(strings.TrimSuffix((baseUrl), "/"))
	if err != nil {
		return err
	}

	log.Debug("DELETE", slog.String("url", u.String()))

	token := authz.Token(ctx)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u.String(), nil)
	if err != nil {
		err = fmt.Errorf("failed to create http request: %w", err)
		return err
	}

	req.Header.Add("Authorization", "Bearer "+token)

	transport := http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := http.Client{
		Transport: otelhttp.NewTransport(&transport),
	}

	resp, err := client.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to delete: %w", err)
		return err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		err = fmt.Errorf("request failed, not authorized")
		return err
	}

	if !slices.Contains([]int{http.StatusNoContent, http.StatusOK}, resp.StatusCode) {
		err = fmt.Errorf("request failed with status code %d", resp.StatusCode)
		return err
	}

	return nil
}
