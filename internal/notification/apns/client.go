package apns

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	ProductionURL  = "https://api.push.apple.com"
	DevelopmentURL = "https://api.sandbox.push.apple.com"
)

// Notification represents an APNs notification
type Notification struct {
	DeviceToken string
	Title       string
	Body        string
	Subtitle    string
	Sound       string
	Badge       *int
	Category    string
	ThreadID    string
	Data        map[string]interface{}
}

// Client is an APNs client
type Client struct {
	httpClient  *http.Client
	keyID       string
	teamID      string
	bundleID    string
	privateKey  *ecdsa.PrivateKey
	isProduction bool

	// JWT token caching
	tokenMu    sync.RWMutex
	token      string
	tokenExpiry time.Time
}

// Config holds APNs configuration
type Config struct {
	KeyID        string
	TeamID       string
	PrivateKey   string
	BundleID     string
	IsProduction bool
}

// NewClient creates a new APNs client
func NewClient(config Config) (*Client, error) {
	privateKey, err := parsePrivateKey(config.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		keyID:        config.KeyID,
		teamID:       config.TeamID,
		bundleID:     config.BundleID,
		privateKey:   privateKey,
		isProduction: config.IsProduction,
	}, nil
}

// Send sends a notification to a device
func (c *Client) Send(ctx context.Context, notification Notification) error {
	payload := c.buildPayload(notification)

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	url := c.getURL() + "/3/device/" + notification.DeviceToken

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	token, err := c.getToken()
	if err != nil {
		return fmt.Errorf("failed to get auth token: %w", err)
	}

	req.Header.Set("Authorization", "bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apns-topic", c.bundleID)
	req.Header.Set("apns-push-type", "alert")
	req.Header.Set("apns-priority", "10")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("APNs error: %s - %s", resp.Status, string(body))
	}

	return nil
}

// SendSilent sends a silent/background notification
func (c *Client) SendSilent(ctx context.Context, deviceToken string) error {
	return c.SendData(ctx, deviceToken, map[string]string{"type": "sync"})
}

// SendData sends a silent/background notification with custom data
func (c *Client) SendData(ctx context.Context, deviceToken string, data map[string]string) error {
	payload := map[string]interface{}{
		"aps": map[string]interface{}{
			"content-available": 1,
		},
	}

	// Add custom data to payload
	for k, v := range data {
		payload[k] = v
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	url := c.getURL() + "/3/device/" + deviceToken

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	token, err := c.getToken()
	if err != nil {
		return fmt.Errorf("failed to get auth token: %w", err)
	}

	req.Header.Set("Authorization", "bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apns-topic", c.bundleID)
	req.Header.Set("apns-push-type", "background")

	// Use high priority for cross-device actions to ensure quick delivery
	// so notifications/alarms are dismissed promptly on all devices
	priority := "5"
	if data["type"] == "cross_device_action" {
		priority = "10"
	}
	req.Header.Set("apns-priority", priority)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("APNs error: %s - %s", resp.Status, string(body))
	}

	return nil
}

func (c *Client) buildPayload(notification Notification) map[string]interface{} {
	alert := map[string]interface{}{
		"title": notification.Title,
		"body":  notification.Body,
	}

	if notification.Subtitle != "" {
		alert["subtitle"] = notification.Subtitle
	}

	aps := map[string]interface{}{
		"alert":            alert,
		"mutable-content":  1,
		"content-available": 1,
	}

	if notification.Sound != "" {
		aps["sound"] = notification.Sound
	}

	if notification.Badge != nil {
		aps["badge"] = *notification.Badge
	}

	if notification.Category != "" {
		aps["category"] = notification.Category
	}

	if notification.ThreadID != "" {
		aps["thread-id"] = notification.ThreadID
	}

	payload := map[string]interface{}{
		"aps": aps,
	}

	// Add custom data
	for k, v := range notification.Data {
		payload[k] = v
	}

	return payload
}

func (c *Client) getToken() (string, error) {
	c.tokenMu.RLock()
	if c.token != "" && time.Now().Before(c.tokenExpiry) {
		token := c.token
		c.tokenMu.RUnlock()
		return token, nil
	}
	c.tokenMu.RUnlock()

	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	// Double-check after acquiring write lock
	if c.token != "" && time.Now().Before(c.tokenExpiry) {
		return c.token, nil
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss": c.teamID,
		"iat": now.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = c.keyID

	signedToken, err := token.SignedString(c.privateKey)
	if err != nil {
		return "", err
	}

	c.token = signedToken
	c.tokenExpiry = now.Add(50 * time.Minute) // APNs tokens are valid for 1 hour

	return signedToken, nil
}

func (c *Client) getURL() string {
	if c.isProduction {
		return ProductionURL
	}
	return DevelopmentURL
}

func parsePrivateKey(pemString string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemString))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	ecdsaKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not an ECDSA private key")
	}

	return ecdsaKey, nil
}
