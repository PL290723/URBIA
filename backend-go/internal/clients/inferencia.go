package clients

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// InferenciaClient habla con el servicio Python de inferencia.
// Contrato: el servicio expone GET /health y responde 200 cuando está listo.
type InferenciaClient struct {
	baseURL string
	http    *http.Client
}

func NewInferenciaClient(baseURL string, timeout time.Duration) *InferenciaClient {
	return &InferenciaClient{baseURL: baseURL, http: &http.Client{Timeout: timeout}}
}

func (c *InferenciaClient) Configurado() bool { return c.baseURL != "" }

func (c *InferenciaClient) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("el servicio de inferencia respondió %d", resp.StatusCode)
	}
	return nil
}
