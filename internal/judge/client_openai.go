package judge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIClient implements LLMClient using the OpenAI Chat Completions API.
type OpenAIClient struct {
	apiKey    string
	baseURL   string
	model     string
	maxTokens int
	client    *http.Client
}

// NewOpenAIClient creates a client for the OpenAI Chat Completions API.
func NewOpenAIClient(apiKey, baseURL, model string, maxTokens int) *OpenAIClient {
	return &OpenAIClient{
		apiKey:    apiKey,
		baseURL:   baseURL,
		model:     model,
		maxTokens: maxTokens,
		client:    &http.Client{Timeout: 120 * time.Second},
	}
}

type openaiRequest struct {
	Model          string           `json:"model"`
	MaxTokens      int              `json:"max_tokens"`
	Messages       []openaiMessage  `json:"messages"`
	ResponseFormat *openaiRespFmt   `json:"response_format,omitempty"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiRespFmt struct {
	Type string `json:"type"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (c *OpenAIClient) Complete(ctx context.Context, systemPrompt, userPrompt string) (*LLMResponse, error) {
	endpoint := c.baseURL + "/chat/completions"

	reqBody := openaiRequest{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		Messages: []openaiMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		ResponseFormat: &openaiRespFmt{Type: "json_object"},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var openaiResp openaiResponse
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	if openaiResp.Error != nil {
		return nil, fmt.Errorf("openai error: %s: %s", openaiResp.Error.Type, openaiResp.Error.Message)
	}

	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from openai")
	}

	return &LLMResponse{Content: openaiResp.Choices[0].Message.Content}, nil
}
