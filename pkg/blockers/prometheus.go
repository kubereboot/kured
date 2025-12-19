package blockers

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"time"

	papi "github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"log/slog"
)

// Compile-time checks to ensure the type implements the interface
var (
	_ RebootBlocker = (*PrometheusBlockingChecker)(nil)
)

// PrometheusBlockingChecker contains info for connecting
// to prometheus, and can give info about whether a reboot should be blocked
type PrometheusBlockingChecker struct {
	promConfig papi.Config
	// regexp used to get alerts
	filter *regexp.Regexp
	// bool to indicate if only firing alerts should be considered
	firingOnly bool
	// bool to indicate that we're only blocking on alerts which match the filter
	filterMatchOnly bool
	// storing the promClient
	promClient papi.Client
}

// NewPrometheusBlockingChecker creates a new PrometheusBlockingChecker using the given
// Prometheus API config, alert filter, and filtering options.
func NewPrometheusBlockingChecker(config papi.Config, alertFilter *regexp.Regexp, firingOnly bool, filterMatchOnly bool) PrometheusBlockingChecker {
	promClient, _ := papi.NewClient(config)

	return PrometheusBlockingChecker{
		promConfig:      config,
		filter:          alertFilter,
		firingOnly:      firingOnly,
		filterMatchOnly: filterMatchOnly,
		promClient:      promClient,
	}
}

// IsBlocked for the prometheus will check if there are active alerts matching
// the arguments given into the PrometheusBlockingChecker which would actively
// block the reboot.
// As of today, no blocker information is shared as a return of the method,
// and the information is simply logged.
func (pb PrometheusBlockingChecker) IsBlocked() bool {
	alertNames, err := pb.ActiveAlerts()
	if err != nil {
		slog.Info("Reboot blocked: prometheus query error", "error", err)
		return true
	}
	count := len(alertNames)
	if count > 10 {
		alertNames = append(alertNames[:10], "...")
	}
	if count > 0 {
		slog.Info(fmt.Sprintf("Reboot blocked: %d active alerts: %v", count, alertNames))
		return true
	}
	return false
}

// MetricLabel is used to give a fancier name
// than the type to the label for rebootBlockedCounter
func (pb PrometheusBlockingChecker) MetricLabel() string {
	return "prometheus_alert"
}

// ActiveAlerts is a method of type promClient, it returns a list of names of active alerts
// (e.g. pending or firing), filtered by the supplied regexp or by the includeLabels query.
// filter by regexp means when the regexp finds the alert-name; the alert is excluded from the
// block-list and will NOT block rebooting. query by includeLabel means,
// if the query finds an alert, it will include it to the block-list, and it WILL block rebooting.
func (pb PrometheusBlockingChecker) ActiveAlerts() ([]string, error) {
	api := v1.NewAPI(pb.promClient)

	// get all alerts from prometheus
	value, _, err := api.Query(context.Background(), "ALERTS", time.Now())
	if err != nil {
		return nil, err
	}

	if value.Type() == model.ValVector {
		if vector, ok := value.(model.Vector); ok {
			activeAlertSet := make(map[string]bool)
			for _, sample := range vector {
				if alertName, isAlert := sample.Metric[model.AlertNameLabel]; isAlert && sample.Value != 0 {
					if matchesRegex(pb.filter, string(alertName), pb.filterMatchOnly) && (!pb.firingOnly || sample.Metric["alertstate"] == "firing") {
						activeAlertSet[string(alertName)] = true
					}
				}
			}

			var activeAlerts []string
			for activeAlert := range activeAlertSet {
				activeAlerts = append(activeAlerts, activeAlert)
			}
			sort.Strings(activeAlerts)

			return activeAlerts, nil
		}
	}

	return nil, fmt.Errorf("unexpected value type %v", value)
}

func matchesRegex(filter *regexp.Regexp, alertName string, filterMatchOnly bool) bool {
	if filter == nil {
		return true
	}

	return filter.MatchString(alertName) == filterMatchOnly
}
