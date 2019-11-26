package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var (
	httpClient = &http.Client{Timeout: 5 * time.Second}
)

type body struct {
	Text     string `json:"text,omitempty"`
	Username string `json:"username,omitempty"`
	Channel  string `json:"channel,omitempty"`
}

func notify(hookURL, username, channel, message string) error {
	msg := body{
		Text:     message,
		Username: username,
		Channel:  channel,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&msg); err != nil {
		return err
	}

	resp, err := httpClient.Post(hookURL, "application/json", &buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf(resp.Status)
	}

	return nil
}

func NotifyDrain(hookURL, username, channel, nodeID string) error {
	return notify(hookURL, username, channel, fmt.Sprintf("Draining node %s", nodeID))
}

func NotifyReboot(hookURL, username, channel, nodeID string) error {
	return notify(hookURL, username, channel, fmt.Sprintf("Rebooting node %s", nodeID))
}
