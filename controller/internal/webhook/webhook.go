package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	log "sigs.k8s.io/controller-runtime/pkg/log"
)

type WebhookManager interface {
	SendWebhook(url string, payload []byte) error
	Listen(ctx context.Context) error
}

type WebhookPayload struct {
	Url       string
	VerifySSL bool
	Data      TaskHookDetails
}

type TaskHookDetails struct {
	Status   string `json:"status"`
	DagRunId int    `json:"dagRunId"`
	TaskName string `json:"taskName"`
}

type webhookManager struct {
	urlvalidator SSLVerifier
	webhookChan  chan WebhookPayload
	client       *http.Client
}

func NewWebhookManager(channel chan WebhookPayload) WebhookManager {
	return &webhookManager{
		webhookChan:  channel,
		client:       &http.Client{},
		urlvalidator: NewSystemURLValidator(),
	}
}

func (w *webhookManager) SendWebhook(url string, payload []byte) error {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send webhook, status code: %d", resp.StatusCode)
	}

	return nil
}

func (w *webhookManager) Listen(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			close(w.webhookChan)
			return ctx.Err()
		case payload := <-w.webhookChan:
			if payload.VerifySSL {
				if err := w.urlvalidator.VerifySSL(payload.Url); err != nil {
					log.Log.Error(err, "invalid url", "url", payload.Url)
					continue
				}
			}

			bytes, err := json.Marshal(payload.Data)
			if err != nil {
				log.Log.Error(err, "Failed to marshal webhook payload:")
				continue
			}

			if err := w.SendWebhook(payload.Url, bytes); err != nil {
				log.Log.Error(err, "failed to send webhook", "url", payload.Url)
				continue
			}

			log.Log.Info("Webhook sent successfully", "url", payload.Url)
		}
	}
}
