package helpers

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type versionKeyType string

const versionKey versionKeyType = "version"

func Decorate(ctx context.Context, kv ...any) context.Context {
	for kvidx := range len(kv) / 2 {
		ctx = context.WithValue(ctx, kv[kvidx*2], kv[(kvidx*2)+1])
	}
	return ctx
}

func WithVersion(ctx context.Context, version string) context.Context {
	ctx = context.WithValue(ctx, versionKey, version)
	return ctx
}

func GetVersion(ctx context.Context) string {
	v := ctx.Value(versionKey)
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func FromCtxInt(ctx context.Context, key any, defaultValue int) int {
	v := ctx.Value(key)
	if v == nil {
		return defaultValue
	}
	if i, ok := v.(int); ok {
		return i
	}
	if s, ok := v.(string); ok {
		i, _ := strconv.Atoi(s)
		return i
	}
	return defaultValue
}

func IsHxRequest(r *http.Request) bool {
	isHxRequest := r.Header.Get("HX-Request")
	return isHxRequest == "true"
}

func UrlParamOrDefault(r *http.Request, param, defaultValue string) string {
	value := r.URL.Query().Get(param)
	if value == "" {
		return defaultValue
	}
	return value
}

func GetOffsetAndLimit(r *http.Request) (offset, limit int) {
	pageIndex := UrlParamOrDefault(r, "page", "1")
	pageSize := UrlParamOrDefault(r, "limit", "15")

	limit, _ = strconv.Atoi(pageSize)
	index, _ := strconv.Atoi(pageIndex)

	if index == 1 {
		offset = 0
		return
	}

	offset = (index - 1) * limit
	return
}

func PagerIndexes(pageIndex, pageCount int) []int64 {
	start := int64(pageIndex)
	last := int64(pageCount)

	const PagerWidth int64 = 6

	start -= (PagerWidth / 2)

	if start > (last - PagerWidth) {
		start = last - PagerWidth
	}

	if start < 1 {
		start = 1
	}

	result := []int64{}

	if start != 1 {
		start = start + 1
		result = append(result, 1, start)
	} else {
		result = append(result, 1)
	}

	page := start + 1

	for len(result) < int(PagerWidth) {
		if page >= last {
			break
		}

		result = append(result, page)
		page = page + 1
	}

	if result[len(result)-1] < last {
		result = append(result, last)
	}

	return result
}

func SanitizeParams(params url.Values, keys ...string) {
	if len(keys) > 0 {
		for _, k := range keys {
			params.Del(k)
		}
	}

	for k, v := range params {
		for i := 0; i < len(v); i++ {
			if v[i] == "" {
				v = append(v[:i], v[i+1:]...)
				i--
			}
		}

		for i := 0; i < len(v); i++ {
			for j := i + 1; j < len(v); j++ {
				if v[i] == v[j] {
					v = append(v[:j], v[j+1:]...)
					j--
				}
			}
		}

		if len(v) == 0 {
			params.Del(k)
		} else {
			params[k] = v
		}
	}
}

func WriteComponentResponse(ctx context.Context, w http.ResponseWriter, r *http.Request, component templ.Component, sizeHint int, cacheTime time.Duration) {
	var writer io.Writer
	var gzipWriter *gzip.Writer

	writeBuffer := bytes.NewBuffer(make([]byte, 0, sizeHint))
	writer = writeBuffer

	isGzipAccepted := func() bool {
		for _, enc := range r.Header["Accept-Encoding"] {
			if strings.Contains(enc, "gzip") {
				return true
			}
		}
		return false
	}()

	if isGzipAccepted && sizeHint > 2000 {
		gzipWriter = gzip.NewWriter(writeBuffer)
		writer = gzipWriter
	}

	err := component.Render(ctx, writer)
	if err != nil {
		logging.GetFromContext(ctx).Error("failed to render templ component", "err", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	if gzipWriter != nil {
		w.Header().Set("Content-Encoding", "gzip")
		gzipWriter.Flush()
	}

	if cacheTime.Seconds() > 1.0 {
		w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", int(math.Round(cacheTime.Seconds()))))
		w.Header().Add("Vary", "Accept-Language")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}

	w.Header().Set("Content-Length", fmt.Sprintf("%d", writeBuffer.Len()))
	w.WriteHeader(http.StatusOK)

	w.Write(writeBuffer.Bytes())
}

func GET(ctx context.Context, targetUrl string, headers map[string][]string, params url.Values) ([]byte, error) {
	u, err := url.Parse(targetUrl)
	if err != nil {
		return nil, fmt.Errorf("could not parse url: %s", err.Error())
	}

	u.RawQuery = params.Encode()

	urlToGet := u.String()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlToGet, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %s", err.Error())
	}

	req.Header = headers

	var httpClient = http.Client{
		Transport: otelhttp.NewTransport(&http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}),
		Timeout: 60 * time.Second,
	}

	response, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send get request: %s", err.Error())
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("request failed: %d", response.StatusCode)
	}

	b, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %s", err.Error())
	}

	return b, nil
}

func FileUpload(ctx context.Context, targetUrl string, headers map[string][]string, f io.Reader) error {
	log := logging.GetFromContext(ctx)

	u, err := url.Parse(targetUrl)
	if err != nil {
		return fmt.Errorf("could not parse url: %s", err.Error())
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fileWriter, err := writer.CreateFormFile("fileupload", "export.csv")
	if err != nil {
		log.Error("Error creating form file", "err", err.Error())
		return err
	}
	_, err = io.Copy(fileWriter, f)
	if err != nil {
		log.Error("Error copying file content", "err", err.Error())
		return err
	}
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), &body)
	if err != nil {
		log.Error("failed to create http request", "err", err.Error())
		return err
	}

	req.Header = headers
	req.Header.Set("Content-Type", writer.FormDataContentType())

	var httpClient = http.Client{
		Transport: otelhttp.NewTransport(&http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}),
		Timeout: 60 * time.Second,
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Error("failed to send post request", "err", err.Error())
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("request failed: %d", resp.StatusCode)
	}

	return nil
}
