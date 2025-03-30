package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	log "sigs.k8s.io/controller-runtime/pkg/log"
)

type WebhookManager interface {
	SendWebhook(url string, payload []byte) error
	Listen(ctx context.Context) error
}

type WebhookNotifier interface {
	NotifyTaskRun(name string, status string, dagRunId, taskId int, url string, verifySSL bool)
	NotifyPodEvent(name string, status string, dagRunId, taskId int, url string, verifySSL bool, duration int)
}

type WebhookDataBase struct {
	// Base struct for webhook data
	Type string `json:"type"`
}

type WebhookPayload struct {
	URL       string
	VerifySSL bool
	Data      any
}

type TaskHookDetails struct {
	WebhookDataBase
	Status   string `json:"status"`
	DagRunId int    `json:"dagRunId"`
	TaskName string `json:"taskName"`
	TaskId   int    `json:"taskId"`
}

type PodEventDetails struct {
	WebhookDataBase
	Status   string `json:"status"`
	DagRunId int    `json:"dagRunId"`
	TaskName string `json:"taskName"`
	TaskId   int    `json:"taskId"`
	Duration int    `json:"duration"`
}

type webhookManager struct {
	urlValidator SSLVerifier
	webhookChan  chan WebhookPayload
	client       *http.Client
}

type webhookNotifier struct {
	webhookChan chan WebhookPayload
}

func NewWebhookManager(channel chan WebhookPayload) WebhookManager {
	return &webhookManager{
		webhookChan:  channel,
		client:       &http.Client{Timeout: 10 * time.Second}, // Set a timeout for HTTP client
		urlValidator: NewSystemURLValidator(),
	}
}

func NewWebhookNotifier(webhookChan chan WebhookPayload) WebhookNotifier {
	return &webhookNotifier{webhookChan: webhookChan}
}

func (w *webhookNotifier) NotifyTaskRun(name string, status string, dagRunId, taskId int, url string, verifySSL bool) {
	w.webhookChan <- WebhookPayload{
		URL:       url,
		VerifySSL: verifySSL,
		Data: TaskHookDetails{
			WebhookDataBase: WebhookDataBase{
				Type: "taskrun",
			},
			Status:   status,
			DagRunId: dagRunId,
			TaskName: name,
			TaskId:   taskId,
		},
	}
}

func (w *webhookNotifier) NotifyPodEvent(name string, status string, dagRunId, taskId int, url string, verifySSL bool, duration int) {
	w.webhookChan <- WebhookPayload{
		URL:       url,
		VerifySSL: verifySSL,
		Data: PodEventDetails{
			WebhookDataBase: WebhookDataBase{
				Type: "pod",
			},
			Status:   status,
			DagRunId: dagRunId,
			TaskName: name,
			TaskId:   taskId,
			Duration: duration,
		},
	}
}

func (w *webhookManager) SendWebhook(url string, payload []byte) error {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send webhook, status code: %d", resp.StatusCode)
	}

	return nil
}

func (w *webhookManager) Listen(ctx context.Context) error {
	defer func() {
		closeChannel(w.webhookChan) // Graceful channel closure
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case payload, ok := <-w.webhookChan:
			if !ok {
				log.Log.Info("Webhook channel closed")
				return nil
			}

			// Validate SSL if required
			if payload.VerifySSL {
				if err := w.urlValidator.VerifySSL(payload.URL); err != nil {
					log.Log.Error(err, "Invalid URL", "url", payload.URL)
					continue
				}
			}

			// Marshal payload data
			data, err := json.Marshal(payload.Data)
			if err != nil {
				log.Log.Error(err, "Failed to marshal webhook payload")
				continue
			}

			// Send webhook
			if err := w.SendWebhook(payload.URL, data); err != nil {
				log.Log.Error(err, "Failed to send webhook", "url", payload.URL)
				continue
			}

			log.Log.Info("Webhook sent successfully", "url", payload.URL)
		}
	}
}

func closeChannel(ch chan WebhookPayload) {
	defer func() {
		if r := recover(); r != nil {
			log.Log.Error(errors.New("panic recovered"), "Attempt to close already closed channel")
		}
	}()
	close(ch)
}
