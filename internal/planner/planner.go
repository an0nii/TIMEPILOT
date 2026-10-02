package planner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"timepilot/internal/config"
	"timepilot/internal/task"
)

var ErrDisabled = errors.New("AI planner is disabled")

type Request struct {
	Goal             string
	AvailableMinutes int
	Tasks            []task.Task
}

type Info struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

type Planner interface {
	Plan(context.Context, Request) (string, error)
	Info() Info
}

type disabled struct{}

func Disabled() Planner { return disabled{} }

func (disabled) Plan(context.Context, Request) (string, error) { return "", ErrDisabled }
func (disabled) Info() Info                                    { return Info{Enabled: false} }

type client struct {
	provider   string
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

func New(cfg config.AIConfig) (Planner, error) {
	if cfg.Provider == "disabled" {
		return Disabled(), nil
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = map[string]string{
			"openai":    "https://api.openai.com/v1",
			"anthropic": "https://api.anthropic.com/v1",
			"gemini":    "https://generativelanguage.googleapis.com/v1beta",
		}[cfg.Provider]
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("invalid AI_BASE_URL: %w", err)
	}
	return &client{
		provider:   cfg.Provider,
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}, nil
}

func (c *client) Info() Info {
	return Info{Enabled: true, Provider: c.provider, Model: c.model}
}

func (c *client) Plan(ctx context.Context, input Request) (string, error) {
	prompt, err := buildPrompt(input)
	if err != nil {
		return "", err
	}
	switch c.provider {
	case "openai":
		return c.openAI(ctx, prompt)
	case "anthropic":
		return c.anthropic(ctx, prompt)
	case "gemini":
		return c.gemini(ctx, prompt)
	default:
		return "", fmt.Errorf("unsupported AI provider %q", c.provider)
	}
}

const systemInstruction = `Ты помощник по тайм-менеджменту. Составляй реалистичный план на русском языке. Учитывай доступное время, сначала незавершённые задачи, добавляй короткие перерывы и не придумывай факты или дедлайны. Верни только компактный план в Markdown.`

func buildPrompt(input Request) (string, error) {
	tasks, err := json.Marshal(input.Tasks)
	if err != nil {
		return "", fmt.Errorf("encode tasks for prompt: %w", err)
	}
	return fmt.Sprintf("Цель пользователя: %s\nДоступно минут: %d\nТекущие задачи JSON: %s", input.Goal, input.AvailableMinutes, tasks), nil
}

func (c *client) openAI(ctx context.Context, prompt string) (string, error) {
	body := map[string]any{
		"model":             c.model,
		"instructions":      systemInstruction,
		"input":             prompt,
		"max_output_tokens": 1000,
		"store":             false,
	}
	var response struct {
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := c.post(ctx, c.baseURL+"/responses", body, map[string]string{
		"Authorization": "Bearer " + c.apiKey,
	}, &response); err != nil {
		return "", err
	}
	var parts []string
	for _, output := range response.Output {
		for _, content := range output.Content {
			if content.Type == "output_text" && content.Text != "" {
				parts = append(parts, content.Text)
			}
		}
	}
	return requireText(parts)
}

func (c *client) anthropic(ctx context.Context, prompt string) (string, error) {
	body := map[string]any{
		"model":      c.model,
		"max_tokens": 1000,
		"system":     systemInstruction,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	var response struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := c.post(ctx, c.baseURL+"/messages", body, map[string]string{
		"x-api-key":         c.apiKey,
		"anthropic-version": "2023-06-01",
	}, &response); err != nil {
		return "", err
	}
	var parts []string
	for _, content := range response.Content {
		if content.Type == "text" && content.Text != "" {
			parts = append(parts, content.Text)
		}
	}
	return requireText(parts)
}

func (c *client) gemini(ctx context.Context, prompt string) (string, error) {
	body := map[string]any{
		"system_instruction": map[string]any{
			"parts": []map[string]string{{"text": systemInstruction}},
		},
		"contents": []map[string]any{{
			"role":  "user",
			"parts": []map[string]string{{"text": prompt}},
		}},
		"generationConfig": map[string]int{"maxOutputTokens": 1000},
	}
	var response struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	endpoint := c.baseURL + "/models/" + url.PathEscape(c.model) + ":generateContent"
	if err := c.post(ctx, endpoint, body, map[string]string{"x-goog-api-key": c.apiKey}, &response); err != nil {
		return "", err
	}
	var parts []string
	for _, candidate := range response.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				parts = append(parts, part.Text)
			}
		}
	}
	return requireText(parts)
}

func (c *client) post(ctx context.Context, endpoint string, payload any, headers map[string]string, output any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode %s request: %w", c.provider, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create %s request: %w", c.provider, err)
	}
	request.Header.Set("Content-Type", "application/json")
	for name, value := range headers {
		request.Header.Set(name, value)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("call %s: %w", c.provider, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("%s returned HTTP %d: %s", c.provider, response.StatusCode, strings.TrimSpace(string(message)))
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(output); err != nil {
		return fmt.Errorf("decode %s response: %w", c.provider, err)
	}
	return nil
}

func requireText(parts []string) (string, error) {
	text := strings.TrimSpace(strings.Join(parts, "\n"))
	if text == "" {
		return "", errors.New("AI provider returned no text")
	}
	return text, nil
}
