package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Notifier struct {
	webhookURL string
	client     *http.Client
}

func New(webhookURL string) *Notifier {
	return &Notifier{webhookURL: webhookURL, client: &http.Client{Timeout: 10 * time.Second}}
}

func (n *Notifier) Notify(ctx context.Context, message string) error {
	body, err := json.Marshal(map[string]string{"text": message})
	if err != nil {
		return fmt.Errorf("slack: encode payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("slack: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("slack: post webhook: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack: webhook returned status %d", resp.StatusCode)
	}
	return nil
}
