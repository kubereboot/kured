package alerts

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/prometheus/client_golang/api/prometheus"
	"github.com/prometheus/common/model"
)

// Returns a list of names of active (e.g. pending or firing) alerts, filtered
// by the supplied regexp.
func PrometheusActiveAlerts(prometheusURL string, filter *regexp.Regexp) ([]string, error) {
	client, err := prometheus.New(prometheus.Config{Address: prometheusURL})
	if err != nil {
		return nil, err
	}

	queryAPI := prometheus.NewQueryAPI(client)

	value, err := queryAPI.Query(context.Background(), "ALERTS", time.Now())
	if err != nil {
		return nil, err
	}

	if value.Type() == model.ValVector {
		if vector, ok := value.(model.Vector); ok {
			var activeAlerts []string
			for _, sample := range vector {
				if alertName, isAlert := sample.Metric[model.AlertNameLabel]; isAlert && sample.Value != 0 {
					if filter == nil || !filter.MatchString(string(alertName)) {
						activeAlerts = append(activeAlerts, string(alertName))
					}
				}
			}
			return activeAlerts, nil
		}
	}

	return nil, fmt.Errorf("Unexpected value type: %v", value)
}
