package alerts

import (
	"context"
	"regexp"
	"sort"

	"github.com/prometheus/alertmanager/client"
	"github.com/prometheus/client_golang/api"
)

// Returns a list of names of active (e.g. pending or firing) alerts, filtered
// by the supplied regexp.
func AlertmanagerActiveAlerts(alertmanagerURL string, filter *regexp.Regexp) ([]string, error) {

	c, err := api.NewClient(api.Config{Address: alertmanagerURL})
	if err != nil {
		return nil, err
	}
	alertAPI := client.NewAlertAPI(c)

	fetchedAlerts, err := alertAPI.List(context.Background(), "", "", false, false, true, false)

	if err != nil {
		return nil, err
	}

	activeAlertSet := make(map[string]bool)

	for _, alert := range fetchedAlerts {
		alertName := alert.Labels["alertname"]
		if filter == nil || !filter.MatchString(string(alertName)) {
			activeAlertSet[string(alertName)] = true
		}
	}

	var activeAlerts []string
	for activeAlert, _ := range activeAlertSet {
		activeAlerts = append(activeAlerts, activeAlert)
	}
	sort.Sort(sort.StringSlice(activeAlerts))

	return activeAlerts, nil

}
