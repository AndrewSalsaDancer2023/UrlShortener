package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GenerateResponse — ответ микросервиса генерации ID.
type GenerateResponse struct {
	NumericID int64  `json:"numeric_id"`
	ShortCode string `json:"short_code"`
}

// IDServiceClient — HTTP-клиент к микросервису генерации ID.
// Инкапсулирует всю логику общения с upstream:
// адрес, таймаут, разбор ответа и ошибок.
type IDServiceClient struct {
	baseURL    string
	httpClient *http.Client
}

// New создаёт клиент с заданным адресом upstream и таймаутом.
func New(baseURL string, timeout time.Duration) *IDServiceClient {
	return &IDServiceClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
			// Переиспользуем соединения — важно для production
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// Generate вызывает POST /api/v1/generate на upstream и возвращает результат.
func (c *IDServiceClient) Generate(ctx context.Context) (*GenerateResponse, error) {
	url := c.baseURL + "/api/v1/generate"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream returned non-200 status: %d", resp.StatusCode)
	}

	var result GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}
