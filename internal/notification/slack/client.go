package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Client is a Slack webhook client
type Client struct {
	httpClient *http.Client
	webhookURL string
}

// message represents a Slack message payload
type message struct {
	Text string `json:"text"`
}

// NewClient creates a new Slack webhook client
func NewClient(webhookURL string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		webhookURL: webhookURL,
	}
}

// send sends a message to the configured Slack channel
func (c *Client) send(ctx context.Context, text string) error {
	payload, err := json.Marshal(message{Text: text})
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.webhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack API error: status %d", resp.StatusCode)
	}

	return nil
}

// SendNewUserNotification sends a notification for a new user signup asynchronously
func (c *Client) SendNewUserNotification(email, displayName string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		text := fmt.Sprintf(":tada: New user signup: *%s* (%s)", displayName, email)
		if err := c.send(ctx, text); err != nil {
			log.Printf("Failed to send Slack notification for new user %s: %v", email, err)
		}
	}()
}
