package ocr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"photo-service-back/domain/photo"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string, timeoutSecondsClient *http.Client) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: timeoutSecondsClient,
	}
}

type recognizeResponse struct {
	Status     string   `json:"status"`
	Bib        *string  `json:"bib"`
	Confidence *float64 `json:"confidence"`
	Error      *string  `json:"error"`
}

func (c *Client) RecognizeBib(ctx context.Context, photoID string, fileName string, file io.Reader) (*photo.BibRecognitionResult, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("ocr service url is empty")
	}

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/recognize-bib", pr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	go func() {
		defer pw.Close()
		defer writer.Close()

		if err := writer.WriteField("photoId", photoID); err != nil {
			_ = pw.CloseWithError(err)
			return
		}

		part, err := writer.CreateFormFile("file", fileName)
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}

		if _, err := io.Copy(part, file); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
	}()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ocr service returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed recognizeResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}

	return &photo.BibRecognitionResult{
		Status:     photo.BibRecognitionStatus(parsed.Status),
		Bib:        parsed.Bib,
		Confidence: parsed.Confidence,
		Error:      parsed.Error,
	}, nil
}
