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
}

func NotifyReboot(hookURL, username, nodeID string) error {
	msg := body{
		Text:     fmt.Sprintf("Rebooting node %s", nodeID),
		Username: username,
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
