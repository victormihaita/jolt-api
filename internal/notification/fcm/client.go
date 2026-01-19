package fcm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"golang.org/x/oauth2/google"
)

const (
	FCMURL = "https://fcm.googleapis.com/v1/projects/%s/messages:send"
)

// Message represents an FCM message
type Message struct {
	Token        string            `json:"token,omitempty"`
	Topic        string            `json:"topic,omitempty"`
	Notification *Notification     `json:"notification,omitempty"`
	Data         map[string]string `json:"data,omitempty"`
	Android      *AndroidConfig    `json:"android,omitempty"`
}

// Notification represents the notification payload
type Notification struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
	Image string `json:"image,omitempty"`
}

// AndroidConfig represents Android-specific configuration
type AndroidConfig struct {
	Priority     string                `json:"priority,omitempty"`
	Notification *AndroidNotification  `json:"notification,omitempty"`
}

// AndroidNotification represents Android notification options
type AndroidNotification struct {
	ChannelID string `json:"channel_id,omitempty"`
	Sound     string `json:"sound,omitempty"`
	Priority  string `json:"notification_priority,omitempty"`
}

// Client is an FCM client
type Client struct {
	httpClient *http.Client
	projectID  string
	credentials []byte

	// Access token caching
	tokenMu     sync.RWMutex
	accessToken string
	tokenExpiry time.Time
}

// Config holds FCM configuration
type Config struct {
	ProjectID      string
	CredentialsJSON string
}

// NewClient creates a new FCM client
func NewClient(config Config) (*Client, error) {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		projectID:   config.ProjectID,
		credentials: []byte(config.CredentialsJSON),
	}, nil
}

// Send sends a notification message to a device
func (c *Client) Send(ctx context.Context, token string, notification *Notification, data map[string]string) error {
	message := Message{
		Token:        token,
		Notification: notification,
		Data:         data,
		Android: &AndroidConfig{
			Priority: "high",
			Notification: &AndroidNotification{
				ChannelID: "reminders",
				Priority:  "PRIORITY_HIGH",
			},
		},
	}

	return c.sendMessage(ctx, message)
}

// SendData sends a data-only message (for Android to handle in background)
func (c *Client) SendData(ctx context.Context, token string, data map[string]string) error {
	message := Message{
		Token: token,
		Data:  data,
		Android: &AndroidConfig{
			Priority: "high",
		},
	}

	return c.sendMessage(ctx, message)
}

// SendToTopic sends a message to a topic
func (c *Client) SendToTopic(ctx context.Context, topic string, notification *Notification, data map[string]string) error {
	message := Message{
		Topic:        topic,
		Notification: notification,
		Data:         data,
	}

	return c.sendMessage(ctx, message)
}

func (c *Client) sendMessage(ctx context.Context, message Message) error {
	payload := map[string]interface{}{
		"message": message,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	url := fmt.Sprintf(FCMURL, c.projectID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	token, err := c.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("FCM error: %s - %s", resp.Status, string(body))
	}

	return nil
}

func (c *Client) getAccessToken(ctx context.Context) (string, error) {
	c.tokenMu.RLock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		token := c.accessToken
		c.tokenMu.RUnlock()
		return token, nil
	}
	c.tokenMu.RUnlock()

	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	// Double-check after acquiring write lock
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}

	// Get credentials from service account
	creds, err := google.CredentialsFromJSON(ctx, c.credentials, "https://www.googleapis.com/auth/firebase.messaging")
	if err != nil {
		return "", fmt.Errorf("failed to parse credentials: %w", err)
	}

	token, err := creds.TokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}

	c.accessToken = token.AccessToken
	c.tokenExpiry = token.Expiry.Add(-1 * time.Minute) // Refresh 1 minute before expiry

	return c.accessToken, nil
}
