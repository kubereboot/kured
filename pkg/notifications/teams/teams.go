package teams

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
	Context string `json:"@context"`
	Type    string `json:"@type"`
	Title   string `json:"title"`
	Text    string `json:"text"`
}

func notify(hookURL, title string, text string) error {
	msg := body{
		Context: "https://schema.org/extensions",
		Type:    "MessageCard",
		Title:   title,
		Text:    text,
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

// NotifyDrain triggers drain notification to Teams channel
func NotifyDrain(hookURL, nodeID string) error {
	return notify(hookURL, "Kured - Drain node", fmt.Sprintf("Draining node %s", nodeID))
}

// NotifyReboot triggers reboot notification to Teams channel
func NotifyReboot(hookURL, nodeID string) error {
	return notify(hookURL, "Kured - Reboot node", fmt.Sprintf("Rebooting node %s", nodeID))
}
