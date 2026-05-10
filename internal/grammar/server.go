// Package grammar – server.go
// HTTP client for an optional local LanguageTool server.
// The server is started by the user (opt-in) via the systemd service
// installed by `make lt-server`.  When available it reduces correction
// latency from ~2 s (CLI cold-start) to ~150 ms (warm HTTP call).
// gramfix falls back to the CLI JAR automatically if the server is down.
package grammar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const serverTimeout = 10 * time.Second

// ltServerResponse mirrors the LanguageTool HTTP API JSON envelope.
type ltServerResponse struct {
	Matches []ltMatch `json:"matches"`
}

// fixViaHTTP sends text to a running LanguageTool HTTP server and returns
// the corrected text.  It uses the same applyCorrections logic as the CLI
// path so behaviour is identical.
func (e *Engine) fixViaHTTP(ctx context.Context, text string) (string, error) {
	if e.cfg.ServerURL == "" {
		return "", fmt.Errorf("no server URL configured")
	}

	apiURL := strings.TrimRight(e.cfg.ServerURL, "/") + "/v2/check"

	form := url.Values{}
	form.Set("text", text)
	form.Set("language", e.cfg.Lang)
	if len(e.cfg.EnabledCategories) > 0 {
		form.Set("enabledCategories", strings.Join(e.cfg.EnabledCategories, ","))
	}
	if len(e.cfg.DisabledRules) > 0 {
		form.Set("disabledRules", strings.Join(e.cfg.DisabledRules, ","))
	}
	if e.cfg.NgramDir != "" {
		// Server reads the ngram path from its startup config, not per-request.
		// We still log it so the user can verify.
	}

	httpCtx, cancel := context.WithTimeout(ctx, serverTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, http.MethodPost, apiURL,
		bytes.NewBufferString(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: serverTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var ltResp ltServerResponse
	if err := json.Unmarshal(body, &ltResp); err != nil {
		return "", fmt.Errorf("parse server JSON: %w", err)
	}

	// Re-use the same patching pipeline as the CLI path.
	resp2 := ltResponse{Matches: ltResp.Matches}
	raw, err := json.Marshal(resp2)
	if err != nil {
		return "", fmt.Errorf("re-marshal: %w", err)
	}
	return applyCorrections(e.cfg, text, raw)
}
