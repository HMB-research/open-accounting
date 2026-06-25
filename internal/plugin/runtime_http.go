package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	BackendRuntimeHTTP = "http"
	maxRuntimeBodySize = 1 << 20
)

var (
	ErrPluginRuntimeUnsupported = errors.New("plugin backend runtime is unsupported")
	ErrPluginRouteNotFound      = errors.New("plugin route is not registered")
	ErrPluginNotEnabled         = errors.New("plugin is not enabled")

	newRuntimeHTTPRequest = http.NewRequestWithContext
)

type runtimeHTTPClient struct {
	baseURL *url.URL
	client  *http.Client
}

type RuntimeRouteResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

type runtimeHookPayload struct {
	PluginID   uuid.UUID `json:"plugin_id"`
	PluginName string    `json:"plugin_name"`
	Handler    string    `json:"handler"`
	Event      Event     `json:"event"`
}

func newRuntimeHTTPClient(backend *BackendConfig) (*runtimeHTTPClient, error) {
	if backend == nil {
		return nil, nil
	}
	if strings.TrimSpace(backend.Runtime) == "" {
		return nil, ErrPluginRuntimeUnavailable
	}
	if !strings.EqualFold(backend.Runtime, BackendRuntimeHTTP) {
		return nil, fmt.Errorf("%w: %s", ErrPluginRuntimeUnsupported, backend.Runtime)
	}
	baseURL, err := parseLoopbackRuntimeBaseURL(backend.BaseURL)
	if err != nil {
		return nil, err
	}
	return &runtimeHTTPClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (c *runtimeHTTPClient) status() PluginRuntimeStatus {
	status := PluginRuntimeStatus{
		Runtime: BackendRuntimeHTTP,
		State:   RuntimeStateExternal,
		Health:  RuntimeHealthUnknown,
		Message: "external HTTP runtime is operator-managed",
	}
	if c != nil && c.baseURL != nil {
		status.BaseURL = c.baseURL.String()
	}
	return status
}

func parseLoopbackRuntimeBaseURL(rawURL string) (*url.URL, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, fmt.Errorf("%w: backend.base_url is required for http runtime", ErrPluginRuntimeUnavailable)
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid backend.base_url: %w", err)
	}
	if parsed.Scheme != "http" {
		return nil, fmt.Errorf("backend.base_url must use http loopback")
	}
	host := parsed.Hostname()
	if host == "" {
		return nil, fmt.Errorf("backend.base_url must include a loopback host")
	}
	if !isLoopbackHost(host) {
		return nil, fmt.Errorf("backend.base_url must use a loopback host")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (c *runtimeHTTPClient) invokeHook(ctx context.Context, pluginID uuid.UUID, pluginName string, hook HookConfig, event Event) error {
	payload, err := json.Marshal(runtimeHookPayload{
		PluginID:   pluginID,
		PluginName: pluginName,
		Handler:    hook.Handler,
		Event:      event,
	})
	if err != nil {
		return err
	}
	target := c.handlerURL(hook.Handler, "")
	req, err := newRuntimeHTTPRequest(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Open-Accounting-Plugin-ID", pluginID.String())
	req.Header.Set("X-Open-Accounting-Plugin-Name", pluginName)
	req.Header.Set("X-Open-Accounting-Event", event.Type)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxRuntimeBodySize))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("plugin hook runtime returned status %d", resp.StatusCode)
	}
	return nil
}

func (c *runtimeHTTPClient) invokeRoute(
	ctx context.Context,
	pluginID uuid.UUID,
	tenantID uuid.UUID,
	route RouteConfig,
	method string,
	requestPath string,
	rawQuery string,
	sourceHeader http.Header,
	body io.Reader,
) (*RuntimeRouteResponse, error) {
	payload, err := io.ReadAll(io.LimitReader(body, maxRuntimeBodySize+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > maxRuntimeBodySize {
		return nil, fmt.Errorf("plugin runtime request body exceeds %d bytes", maxRuntimeBodySize)
	}

	target := c.handlerURL(route.Handler, rawQuery)
	req, err := newRuntimeHTTPRequest(ctx, method, target, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	copyRuntimeRequestHeaders(req.Header, sourceHeader)
	req.Header.Set("X-Open-Accounting-Plugin-ID", pluginID.String())
	req.Header.Set("X-Open-Accounting-Tenant-ID", tenantID.String())
	req.Header.Set("X-Open-Accounting-Route-Path", requestPath)
	req.Header.Set("X-Open-Accounting-Route-Method", method)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxRuntimeBodySize+1))
	if err != nil {
		return nil, err
	}
	if len(responseBody) > maxRuntimeBodySize {
		return nil, fmt.Errorf("plugin runtime response body exceeds %d bytes", maxRuntimeBodySize)
	}

	return &RuntimeRouteResponse{
		StatusCode: resp.StatusCode,
		Header:     cloneRuntimeResponseHeaders(resp.Header),
		Body:       responseBody,
	}, nil
}

func (c *runtimeHTTPClient) handlerURL(handler string, rawQuery string) string {
	target := *c.baseURL
	handlerPath := strings.TrimSpace(handler)
	if handlerPath == "" {
		handlerPath = "/"
	}
	if !strings.HasPrefix(handlerPath, "/") {
		handlerPath = "/" + handlerPath
	}
	basePath := strings.TrimRight(target.Path, "/")
	target.Path = basePath + handlerPath
	target.RawQuery = rawQuery
	return target.String()
}

func copyRuntimeRequestHeaders(dst, src http.Header) {
	for key, values := range src {
		if isHopByHopHeader(key) || strings.EqualFold(key, "Authorization") {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func cloneRuntimeResponseHeaders(src http.Header) http.Header {
	dst := make(http.Header)
	for key, values := range src {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
	return dst
}

func isHopByHopHeader(key string) bool {
	return slices.Contains([]string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailer",
		"Transfer-Encoding",
		"Upgrade",
	}, http.CanonicalHeaderKey(key))
}
