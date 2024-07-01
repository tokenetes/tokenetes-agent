package configsync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/tratteria/tratteria-agent/verificationrules/v1alpha1"
	"go.uber.org/zap"
)

const (
	MAX_REGISTRATION_ATTEMPTS       = 5
	FAILED_HEARTBEAT_RETRY_INTERVAL = 5 * time.Second
	REGISTRATION_PATH               = "register"
	HEARTBEAT_PATH                  = "heartbeat"
)

type Client struct {
	webhookPort              int
	webhookIP                string
	tconfigdUrl              *url.URL
	serviceName              string
	namespace                string
	verificationRulesManager v1alpha1.VerificationRulesManager
	heartbeatInterval        time.Duration
	httpClient               *http.Client
	logger                   *zap.Logger
}

func NewClient(WebhookPort int, TconfigdUrl *url.URL, ServiceName string, namespace string, VerificationRulesManager v1alpha1.VerificationRulesManager, HeartbeatInterval time.Duration, HttpClient *http.Client, Logger *zap.Logger) (*Client, error) {
	webhookIP, err := getLocalIP()
	if err != nil {
		return nil, err
	}

	return &Client{
		webhookPort:              WebhookPort,
		webhookIP:                webhookIP,
		tconfigdUrl:              TconfigdUrl,
		serviceName:              ServiceName,
		namespace:                namespace,
		verificationRulesManager: VerificationRulesManager,
		heartbeatInterval:        HeartbeatInterval,
		httpClient:               HttpClient,
		logger:                   Logger,
	}, nil
}

type registrationRequest struct {
	IPAddress   string `json:"ipAddress"`
	Port        int    `json:"port"`
	ServiceName string `json:"serviceName"`
	Namespace   string `json:"namespace"`
}

type heartBeatRequest struct {
	IPAddress      string `json:"ipAddress"`
	Port           int    `json:"port"`
	ServiceName    string `json:"serviceName"`
	Namespace      string `json:"namespace"`
	RulesVersionID string `json:"rulesVersionId"`
}

func (c *Client) Start() error {
	if err := c.registerWithBackoff(); err != nil {
		return fmt.Errorf("failed to register with tconfigd: %w", err)
	}

	c.logger.Info("Successfully registered to tconfigd")

	c.logger.Info("Starting heartbeats to tconfigd...")

	go c.startHeartbeat()

	return nil
}

func (c *Client) registerWithBackoff() error {
	var attempt int

	for {
		if err := c.register(); err != nil {
			c.logger.Error("Registration failed", zap.Error(err))

			attempt++

			if attempt >= MAX_REGISTRATION_ATTEMPTS {
				return fmt.Errorf("max registration attempts reached: %w", err)
			}

			backoff := time.Duration(rand.Intn(1<<attempt)) * time.Second

			c.logger.Info("Retrying registration", zap.Duration("backoff", backoff), zap.Int("attempt", attempt))

			time.Sleep(backoff)

			continue
		}

		break
	}

	return nil
}

func (c *Client) register() error {
	registrationReq := registrationRequest{
		IPAddress:   c.webhookIP,
		Port:        c.webhookPort,
		ServiceName: c.serviceName,
		Namespace:   c.namespace,
	}

	jsonData, err := json.Marshal(registrationReq)
	if err != nil {
		return fmt.Errorf("failed to marshal registration data: %w", err)
	}

	registerEndpoint := c.tconfigdUrl.ResolveReference(&url.URL{Path: REGISTRATION_PATH})

	req, err := http.NewRequest(http.MethodPost, registerEndpoint.String(), bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create registration request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send registration request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registration failed with status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) startHeartbeat() {
	heartbeatEndpoint := c.tconfigdUrl.ResolveReference(&url.URL{Path: HEARTBEAT_PATH})

	for {
		heartBeatReq := heartBeatRequest{
			IPAddress:      c.webhookIP,
			Port:           c.webhookPort,
			ServiceName:    c.serviceName,
			Namespace:      c.namespace,
			RulesVersionID: c.verificationRulesManager.GetRulesVersionId(),
		}

		heartBeatRequestJson, err := json.Marshal(heartBeatReq)
		if err != nil {
			c.logger.Error("Failed to marshal heartbeat request", zap.Error(err))
			time.Sleep(FAILED_HEARTBEAT_RETRY_INTERVAL)

			continue
		}

		req, err := http.NewRequest(http.MethodPost, heartbeatEndpoint.String(), bytes.NewBuffer(heartBeatRequestJson))
		if err != nil {
			c.logger.Error("Failed to create heartbeat request", zap.Error(err))
			time.Sleep(FAILED_HEARTBEAT_RETRY_INTERVAL)

			continue
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			c.logger.Error("Failed to send heartbeat", zap.Error(err))
			time.Sleep(FAILED_HEARTBEAT_RETRY_INTERVAL)

			continue
		} else {
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				c.logger.Error("Received non-ok heartbeat response", zap.Int("status", resp.StatusCode))
				time.Sleep(FAILED_HEARTBEAT_RETRY_INTERVAL)

				continue
			} else {
				c.logger.Info("Heartbeat sent successfully")
			}
		}

		time.Sleep(c.heartbeatInterval)
	}
}

func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("couldn't obtain a webhook IP address")
}
