package alerts

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// Returns a list of names of active (e.g. pending or firing) alerts, filtered
// by the supplied regexp.
func PrometheusActiveAlerts(prometheusURL string, filter *regexp.Regexp) ([]string, error) {
	client, err := api.NewClient(api.Config{Address: prometheusURL})
	if err != nil {
		return nil, err
	}

	queryAPI := v1.NewAPI(client)

	value, _, err := queryAPI.Query(context.Background(), "ALERTS", time.Now())
	if err != nil {
		return nil, err
	}

	if value.Type() == model.ValVector {
		if vector, ok := value.(model.Vector); ok {
			activeAlertSet := make(map[string]bool)
			for _, sample := range vector {
				if alertName, isAlert := sample.Metric[model.AlertNameLabel]; isAlert && sample.Value != 0 {
					if filter == nil || !filter.MatchString(string(alertName)) {
						activeAlertSet[string(alertName)] = true
					}
				}
			}

			var activeAlerts []string
			for activeAlert, _ := range activeAlertSet {
				activeAlerts = append(activeAlerts, activeAlert)
			}
			sort.Sort(sort.StringSlice(activeAlerts))

			return activeAlerts, nil
		}
	}

	return nil, fmt.Errorf("Unexpected value type: %v", value)
}
