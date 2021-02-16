package teams

import (
	"fmt"

	"github.com/dasrick/go-teams-notify/v2"
)

func notify(hookURL, message string) error {

	mstClient := goteamsnotify.NewClient()

	msgCard := goteamsnotify.NewMessageCard()
	msgCard.Title = "Node is rebooting"
	msgCard.Text = message
	msgCard.ThemeColor = "#3D4ADF"

	return mstClient.Send(hookURL, msgCard)
}

// NotifyDrain is the exposed way to notify of a drain event onto a slack chan
func NotifyDrain(hookURL, messageTemplate, nodeID string) error {
	return notify(hookURL, fmt.Sprintf(messageTemplate, nodeID))
}

// NotifyReboot is the exposed way to notify of a reboot event onto a slack chan
func NotifyReboot(hookURL, messageTemplate, nodeID string) error {
	return notify(hookURL, fmt.Sprintf(messageTemplate, nodeID))
}
