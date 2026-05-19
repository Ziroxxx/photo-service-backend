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

func NewClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

type recognizedBibResponse struct {
	Bib        string   `json:"bib"`
	Confidence *float64 `json:"confidence"`
}

type recognizeResponse struct {
	Status string `json:"status"`

	// Старый формат ответа OCR-сервиса:
	// {
	//   "status": "completed",
	//   "bib": "247",
	//   "confidence": 0.93
	// }
	Bib        *string  `json:"bib"`
	Confidence *float64 `json:"confidence"`

	// Новый формат ответа OCR-сервиса:
	// {
	//   "status": "completed",
	//   "bibs": [
	//     { "bib": "247", "confidence": 0.93 },
	//     { "bib": "318", "confidence": 0.88 }
	//   ]
	// }
	Bibs []recognizedBibResponse `json:"bibs"`

	Error *string `json:"error"`
}

func (c *Client) RecognizeBib(
	ctx context.Context,
	photoID string,
	fileName string,
	file io.Reader,
) (*photo.BibRecognitionResult, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("ocr service url is empty")
	}

	if c.httpClient == nil {
		return nil, fmt.Errorf("ocr http client is nil")
	}

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/recognize-bib",
		pr,
	)
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

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf(
			"ocr service returned %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	var parsed recognizeResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}

	bibs := make([]photo.RecognizedBib, 0, len(parsed.Bibs))

	for _, item := range parsed.Bibs {
		bibValue := strings.TrimSpace(item.Bib)
		if bibValue == "" {
			continue
		}

		bibs = append(bibs, photo.RecognizedBib{
			Bib:        bibValue,
			Confidence: item.Confidence,
		})
	}

	// Обратная совместимость со старым форматом:
	// если массив bibs не пришёл, но пришло одиночное поле bib,
	// превращаем его в массив из одного элемента.
	if len(bibs) == 0 && parsed.Bib != nil {
		bibValue := strings.TrimSpace(*parsed.Bib)
		if bibValue != "" {
			bibs = append(bibs, photo.RecognizedBib{
				Bib:        bibValue,
				Confidence: parsed.Confidence,
			})
		}
	}

	return &photo.BibRecognitionResult{
		Status:     photo.BibRecognitionStatus(parsed.Status),
		Bib:        parsed.Bib,
		Confidence: parsed.Confidence,
		Bibs:       bibs,
		Error:      parsed.Error,
	}, nil
}
