package notifications

import (
	"errors"
	"github.com/containrrr/shoutrrr"
	"github.com/containrrr/shoutrrr/pkg/router"
	"github.com/containrrr/shoutrrr/pkg/types"
	"log/slog"
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

func NewNotifier(URLs ...string) Notifier {
	if URLs == nil {
		return &NoopNotifier{}
	}
	var servicesURLs []string
	for _, givenURL := range URLs {
		serviceURL := stripQuotes(givenURL)
		if serviceURL != "" {
			servicesURLs = append(servicesURLs, serviceURL)
		}

	}
	if len(servicesURLs) == 0 {
		return &NoopNotifier{}
	}

	sr, err := shoutrrr.CreateSender(servicesURLs...)
	if err != nil {
		slog.Info(
			"Could not create shoutrrr notifier. Will not notify",
			"urls", strings.Join(servicesURLs, " "),
			"error", err,
		)
		return &NoopNotifier{}
	}

	return &ShoutrrrNotifier{serviceRouter: sr}
}

// stripQuotes removes any literal single or double quote chars that surround a string
func stripQuotes(str string) string {
	if len(str) >= 2 {
		firstChar := str[0]
		lastChar := str[len(str)-1]
		if firstChar == lastChar && (firstChar == '"' || firstChar == '\'') {
			return str[1 : len(str)-1]
		}
	}
	// return the original string if it has a length of zero or one
	return str
}
