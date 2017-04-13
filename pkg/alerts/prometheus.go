package alerts

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/prometheus/client_golang/api/prometheus"
	"github.com/prometheus/common/model"
)

// Return true if there are any active (e.g. pending or firing) alerts
func PrometheusCountActive(prometheusURL string, filter *regexp.Regexp) (int, error) {
	client, err := prometheus.New(prometheus.Config{Address: prometheusURL})
	if err != nil {
		return 0, err
	}

	queryAPI := prometheus.NewQueryAPI(client)

	value, err := queryAPI.Query(context.Background(), "ALERTS", time.Now())
	if err != nil {
		return 0, err
	}

	if value.Type() == model.ValVector {
		if vector, ok := value.(model.Vector); ok {
			var count int
			for _, sample := range vector {
				if alertName, isAlert := sample.Metric[model.AlertNameLabel]; isAlert {
					if filter == nil || !filter.MatchString(string(alertName)) {
						count++
					}
				}
			}
			return count, nil
		}
	}

	return 0, fmt.Errorf("Unexpected value type: %v", value)
}
