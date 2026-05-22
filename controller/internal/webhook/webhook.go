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
	NotifyTaskRun(name string, status string, dagRunId, taskID int, url string, verifySSL bool)
	NotifyPodEvent(name string, status string, dagRunId, taskID int, url string, verifySSL bool, duration int)
}

// WebhookDataBase is a base struct for webhook data, containing common fields
type WebhookDataBase struct {
	// Base struct for webhook data
	Type string `json:"type"`
}

// WebhookPayload represents the payload sent to the webhook
type WebhookPayload struct {
	URL       string
	VerifySSL bool
	Data      any
}

// TaskHookDetails represents the details of a task run event to be sent to the webhook
type TaskHookDetails struct {
	WebhookDataBase
	Status   string `json:"status"`
	DagRunId int    `json:"dagRunId"`
	TaskName string `json:"taskName"`
	TaskID   int    `json:"taskId"`
}

// PodEventDetails represents the details of a pod event to be sent to the webhook
type PodEventDetails struct {
	WebhookDataBase
	Status   string `json:"status"`
	DagRunId int    `json:"dagRunId"`
	TaskName string `json:"taskName"`
	TaskID   int    `json:"taskId"`
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

// NewWebhookManager creates a new instance of WebhookManager with the provided channel for receiving webhook payloads
func NewWebhookManager(channel chan WebhookPayload) WebhookManager {
	return &webhookManager{
		webhookChan:  channel,
		client:       &http.Client{Timeout: 10 * time.Second}, // Set a timeout for HTTP client
		urlValidator: NewSystemURLValidator(),
	}
}

// NewWebhookNotifier creates a new instance of WebhookNotifier with the provided channel for sending webhook payloads
func NewWebhookNotifier(webhookChan chan WebhookPayload) WebhookNotifier {
	return &webhookNotifier{webhookChan: webhookChan}
}

func (w *webhookNotifier) NotifyTaskRun(name string, status string, dagRunId, taskID int, url string, verifySSL bool) {
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
			TaskID:   taskID,
		},
	}
}

// NotifyPodEvent sends a webhook notification for a pod event, including the event status, DAG run ID, task name, task ID, and duration of the event
func (w *webhookNotifier) NotifyPodEvent(name string, status string, dagRunId, taskID int, url string, verifySSL bool, duration int) {
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
			TaskID:   taskID,
			Duration: duration,
		},
	}
}

// SendWebhook sends a POST request to the specified URL with the given payload, and returns an error if the request fails or if the response status code is not 200 OK
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send webhook, status code: %d", resp.StatusCode)
	}

	return nil
}

// Listen starts the webhook manager to listen for incoming webhook payloads and process them accordingly
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
