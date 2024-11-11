package notifications

import (
	"errors"
	"fmt"
	"github.com/containrrr/shoutrrr"
	"github.com/containrrr/shoutrrr/pkg/router"
	"github.com/containrrr/shoutrrr/pkg/types"
	"log/slog"
	"net/url"
	"strings"
)

type Notifier interface {
	Send(string, string) error
}

type NoopNotifier struct{}

func (nn *NoopNotifier) Send(_ string, _ string) error {
	return nil
}

type ShoutrrrNotifier struct {
	serviceRouter *router.ServiceRouter
}

func (sn *ShoutrrrNotifier) Send(message string, title string) error {
	params := &types.Params{}
	params.SetTitle(title)
	errs := sn.serviceRouter.Send(message, params)
	var errList error
	if errs != nil {
		for _, err := range errs {
			errList = errors.Join(errList, err)
		}
		return errList
	}
	return nil
}

func NewNotifier(shoutrrrURL string, legacyURL string) Notifier {
	URL := validateNotificationURL(shoutrrrURL, legacyURL)
	switch URL {
	case "":
		return &NoopNotifier{}
	default:
		servicesURLs := strings.Split(URL, ",")
		sr, err := shoutrrr.CreateSender(servicesURLs...)
		if err != nil {
			slog.Info("Could not create shoutrrr notifier. Will not notify", "url", URL)
			return &NoopNotifier{}
		}
		return &ShoutrrrNotifier{serviceRouter: sr}
	}
}

// Remove old flags
func validateNotificationURL(notifyURL string, slackHookURL string) string {
	switch {
	case slackHookURL != "" && notifyURL != "":
		slog.Error(fmt.Sprintf("Cannot use both --notify-url (given: %v) and --slack-hook-url (given: %v) flags. Kured will only use --notify-url flag", slackHookURL, notifyURL))
		return validateNotificationURL(notifyURL, "")
	case notifyURL != "":
		return stripQuotes(notifyURL)
	case slackHookURL != "":
		slog.Info("Deprecated flag(s). Please use --notify-url flag instead.")
		parsedURL, err := url.Parse(stripQuotes(slackHookURL))
		if err != nil {
			slog.Info(fmt.Sprintf("slack-hook-url is not properly formatted... no notification will be sent: %v\n", err))
			return ""
		}
		if len(strings.Split(strings.Replace(parsedURL.Path, "/services/", "", -1), "/")) != 3 {
			slog.Error(fmt.Sprintf("slack-hook-url is not properly formatted... no notification will be sent: unexpected number of / in URL\n"))
			return ""
		}
		return fmt.Sprintf("slack://%s", strings.Replace(parsedURL.Path, "/services/", "", -1))
	}
	return ""
}

// stripQuotes removes any literal single or double quote chars that surround a string
func stripQuotes(str string) string {
	if len(str) > 2 {
		firstChar := str[0]
		lastChar := str[len(str)-1]
		if firstChar == lastChar && (firstChar == '"' || firstChar == '\'') {
			return str[1 : len(str)-1]
		}
	}
	// return the original string if it has a length of zero or one
	return str
}
