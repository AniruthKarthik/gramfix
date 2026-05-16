package grammar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/AniruthKarthik/gramfix/internal/log"
)

const (
	openRouterURL     = "https://openrouter.ai/api/v1/chat/completions"
	openRouterTimeout = 15 * time.Second
)

type openRouterRequest struct {
	Model    string              `json:"model"`
	Messages []openRouterMessage `json:"messages"`
}

type openRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (e *Engine) fixViaOpenRouter(ctx context.Context, text string) (string, error) {
	if e.cfg.OpenRouterAPIKey == "" {
		return "", fmt.Errorf("OpenRouter API key not configured")
	}

	models := []string{
		"deepseek/deepseek-chat-v3-0324:free",
		"qwen/qwen3-235b-a22b:free",
		"meta-llama/llama-3.3-70b-instruct:free",
	}

	// If a specific model is forced via config, try it first
	if e.cfg.OpenRouterModel != "" {
		found := false
		for _, m := range models {
			if m == e.cfg.OpenRouterModel {
				found = true
				break
			}
		}
		if !found {
			models = append([]string{e.cfg.OpenRouterModel}, models...)
		} else {
			// Move it to the front
			newModels := []string{e.cfg.OpenRouterModel}
			for _, m := range models {
				if m != e.cfg.OpenRouterModel {
					newModels = append(newModels, m)
				}
			}
			models = newModels
		}
	}

	var lastErr error
	for _, model := range models {
		log.Debug("trying OpenRouter model: %s", model)
		corrected, err := e.fixWithModel(ctx, text, model)
		if err == nil {
			log.Info("grammar fix via OpenRouter (%s): %d → %d chars", model, len(text), len(corrected))
			log.Audit(text, "OpenRouter API ("+model+")", corrected)
			return corrected, nil
		}
		log.Warn("OpenRouter model %s failed: %v", model, err)
		lastErr = err
	}

	return "", fmt.Errorf("all OpenRouter models failed. last error: %w", lastErr)
}

func (e *Engine) fixWithModel(ctx context.Context, text, model string) (string, error) {
	maxRetries := 2
	backoff := 500 * time.Millisecond

	for i := 0; i <= maxRetries; i++ {
		corrected, err := e.doRequest(ctx, text, model)
		if err == nil {
			return corrected, nil
		}

		// Only retry on transient errors: 429 (rate limit) or 5xx (server error)
		isTransient := false
		if strings.Contains(err.Error(), "HTTP 429") ||
			strings.Contains(err.Error(), "HTTP 500") ||
			strings.Contains(err.Error(), "HTTP 502") ||
			strings.Contains(err.Error(), "HTTP 503") ||
			strings.Contains(err.Error(), "HTTP 504") {
			isTransient = true
		}

		if !isTransient || i == maxRetries {
			return "", err
		}

		log.Debug("OpenRouter model %s transient error: %v, retrying in %v...", model, err, backoff)
		select {
		case <-time.After(backoff):
			backoff *= 2
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	return "", fmt.Errorf("unexpected end of retry loop")
}

func (e *Engine) doRequest(ctx context.Context, text, model string) (string, error) {
	reqBody := openRouterRequest{
		Model: model,
		Messages: []openRouterMessage{
			{
				Role:    "system",
				Content: "You are a professional editor. Correct the grammar, improve the tone and wording naturally, and preserve the original meaning. Avoid robotic rewrites. Respond ONLY with the corrected text, without any explanations or quotes.",
			},
			{
				Role:    "user",
				Content: text,
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpCtx, cancel := context.WithTimeout(ctx, openRouterTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, http.MethodPost, openRouterURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.cfg.OpenRouterAPIKey)
	req.Header.Set("HTTP-Referer", "https://github.com/AniruthKarthik/gramfix")
	req.Header.Set("X-Title", "gramfix")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp openRouterResponse
		json.Unmarshal(body, &errResp)
		if errResp.Error.Message != "" {
			return "", fmt.Errorf("OpenRouter error (HTTP %d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("OpenRouter returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var orResp openRouterResponse
	if err := json.Unmarshal(body, &orResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if len(orResp.Choices) == 0 || orResp.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("empty response from OpenRouter")
	}

	corrected := strings.TrimSpace(orResp.Choices[0].Message.Content)
	// Remove potential surrounding quotes if the model added them despite instructions
	corrected = strings.Trim(corrected, `"'`)

	if err := validateCorrection(text, corrected); err != nil {
		return "", fmt.Errorf("validation failed: %w", err)
	}

	return corrected, nil
}
