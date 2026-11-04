package webhooks

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"
)

type HTTPSender struct{ Client *http.Client }

func NewHTTPSender() *HTTPSender { return &HTTPSender{Client: &http.Client{Timeout: 10 * time.Second}} }

func (s *HTTPSender) Send(ctx context.Context, url string, headers map[string]string, body []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	return resp.StatusCode, nil
}
