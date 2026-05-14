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

	"github.com/anilnair00/gramfix/internal/log"
)

const (
	groqURL     = "https://api.groq.com/openai/v1/chat/completions"
	groqTimeout = 15 * time.Second
)

type groqRequest struct {
	Model    string        `json:"model"`
	Messages []groqMessage `json:"messages"`
}

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (e *Engine) fixViaGroq(ctx context.Context, text string) (string, error) {
	if e.cfg.GroqAPIKey == "" {
		return "", fmt.Errorf("Groq API key not configured")
	}

	models := []string{
		"llama-4-scout-17b-16e",
		"llama-3.3-70b-versatile",
		"llama-3.1-8b-instant",
	}

	// If a specific model is forced via config, try it first
	if e.cfg.GroqModel != "" {
		found := false
		for _, m := range models {
			if m == e.cfg.GroqModel {
				found = true
				break
			}
		}
		if !found {
			models = append([]string{e.cfg.GroqModel}, models...)
		} else {
			// Move it to the front
			newModels := []string{e.cfg.GroqModel}
			for _, m := range models {
				if m != e.cfg.GroqModel {
					newModels = append(newModels, m)
				}
			}
			models = newModels
		}
	}

	var lastErr error
	for _, model := range models {
		log.Debug("trying Groq model: %s", model)
		corrected, err := e.fixWithGroqModel(ctx, text, model)
		if err == nil {
			log.Info("grammar fix via Groq (%s): %d → %d chars", model, len(text), len(corrected))
			log.Audit(text, "Groq API ("+model+")", corrected)
			return corrected, nil
		}
		log.Warn("Groq model %s failed: %v", model, err)
		lastErr = err
	}

	return "", fmt.Errorf("all Groq models failed. last error: %w", lastErr)
}

func (e *Engine) fixWithGroqModel(ctx context.Context, text, model string) (string, error) {
	maxRetries := 2
	backoff := 500 * time.Millisecond

	for i := 0; i <= maxRetries; i++ {
		corrected, err := e.doGroqRequest(ctx, text, model)
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

		log.Debug("Groq model %s transient error: %v, retrying in %v...", model, err, backoff)
		select {
		case <-time.After(backoff):
			backoff *= 2
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	return "", fmt.Errorf("unexpected end of retry loop")
}

func (e *Engine) doGroqRequest(ctx context.Context, text, model string) (string, error) {
	reqBody := groqRequest{
		Model: model,
		Messages: []groqMessage{
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

	httpCtx, cancel := context.WithTimeout(ctx, groqTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, http.MethodPost, groqURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.cfg.GroqAPIKey)

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
		var errResp groqResponse
		json.Unmarshal(body, &errResp)
		if errResp.Error.Message != "" {
			return "", fmt.Errorf("Groq error (HTTP %d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("Groq returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var gResp groqResponse
	if err := json.Unmarshal(body, &gResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if len(gResp.Choices) == 0 || gResp.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("empty response from Groq")
	}

	corrected := strings.TrimSpace(gResp.Choices[0].Message.Content)
	// Remove potential surrounding quotes if the model added them despite instructions
	corrected = strings.Trim(corrected, `"'`)

	if err := validateCorrection(text, corrected); err != nil {
		return "", fmt.Errorf("validation failed: %w", err)
	}

	return corrected, nil
}
