package imaging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type CudaClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewCudaClient(baseURL string, timeout time.Duration) *CudaClient {
	return &CudaClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

type CudaHealthResponse struct {
	Status          string `json:"status"`
	CudaAvailable   bool   `json:"cuda_available"`
	CudaDeviceCount int    `json:"cuda_device_count"`
}

func (c *CudaClient) Health(ctx context.Context) (*CudaHealthResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cuda health failed: status %d", resp.StatusCode)
	}

	var result CudaHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

type CudaBatchRequest struct {
	PreviewMaxWidth   int             `json:"preview_max_width"`
	PreviewMaxHeight  int             `json:"preview_max_height"`
	JPEGQuality       int             `json:"jpeg_quality"`
	WatermarkMaxRatio float64         `json:"watermark_max_ratio"`
	WatermarkOpacity  float64         `json:"watermark_opacity"`
	Padding           int             `json:"padding"`
	Items             []CudaBatchItem `json:"items"`
}

type CudaBatchItem struct {
	PhotoID           string `json:"photo_id"`
	SourceGetURL      string `json:"source_get_url"`
	PreviewPutURL     string `json:"preview_put_url"`
	WatermarkedPutURL string `json:"watermarked_put_url"`
}

type CudaBatchResponse struct {
	Status      string                `json:"status"`
	TotalCount  int                   `json:"total_count"`
	FailedCount int                   `json:"failed_count"`
	Items       []CudaBatchItemResult `json:"items"`
}

type CudaBatchItemResult struct {
	PhotoID     string            `json:"photo_id"`
	Status      string            `json:"status"`
	Error       string            `json:"error,omitempty"`
	Preview     CudaVariantResult `json:"preview,omitempty"`
	Watermarked CudaVariantResult `json:"watermarked,omitempty"`
}

type CudaVariantResult struct {
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	MimeType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
}

func (c *CudaClient) ProcessBatch(ctx context.Context, request CudaBatchRequest) (*CudaBatchResponse, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/process-batch",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result CudaBatchResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if result.Status != "" {
			return nil, fmt.Errorf("cuda process-batch failed: %s", result.Status)
		}
		return nil, fmt.Errorf("cuda process-batch failed: status %d", resp.StatusCode)
	}

	return &result, nil
}
