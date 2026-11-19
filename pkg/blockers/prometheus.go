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
	log "github.com/sirupsen/logrus"
)

// Compile-time checks to ensure the type implements the interface
var (
	_ RebootBlocker = (*PrometheusBlockingChecker)(nil)
)

// DefaultPrometheusQuery is the PromQL query used to look for blocking alerts
// when no custom query is given.
const DefaultPrometheusQuery = "ALERTS"

// PrometheusBlockingChecker contains info for connecting
// to prometheus, and can give info about whether a reboot should be blocked
type PrometheusBlockingChecker struct {
	promConfig papi.Config
	// PromQL query whose non-zero results block the reboot
	query string
	// regexp used to get alerts
	filter *regexp.Regexp
	// bool to indicate if only firing alerts should be considered
	firingOnly bool
	// bool to indicate that we're only blocking on alerts which match the filter
	filterMatchOnly bool
	// storing the promClient
	promClient papi.Client
	initErr    error
}

// NewPrometheusBlockingChecker creates a new PrometheusBlockingChecker using the given
// Prometheus API config, PromQL query, alert filter, and filtering options.
// An empty query defaults to DefaultPrometheusQuery.
func NewPrometheusBlockingChecker(config papi.Config, query string, alertFilter *regexp.Regexp, firingOnly bool, filterMatchOnly bool) PrometheusBlockingChecker {
	promClient, err := papi.NewClient(config)
	if query == "" {
		query = DefaultPrometheusQuery
	}

	return PrometheusBlockingChecker{
		promConfig:      config,
		query:           query,
		filter:          alertFilter,
		firingOnly:      firingOnly,
		filterMatchOnly: filterMatchOnly,
		promClient:      promClient,
		initErr:         err,
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
		log.Warnf("Reboot blocked: prometheus query error: %v", err)
		return true
	}
	count := len(alertNames)
	if count > 10 {
		alertNames = append(alertNames[:10], "...")
	}
	if count > 0 {
		log.Warnf("Reboot blocked: %d active alerts: %v", count, alertNames)
		return true
	}
	return false
}

// MetricLabel is used to give a fancier name
// than the type to the label for rebootBlockedCounter
func (pb PrometheusBlockingChecker) MetricLabel() string {
	return "prometheus"
}

// ActiveAlerts is a method of type promClient, it returns a list of names of active alerts
// (e.g. pending or firing), found by running the configured PromQL query (ALERTS by default),
// filtered by the supplied regexp or by the includeLabels query.
// filter by regexp means when the regexp finds the alert-name; the alert is excluded from the
// block-list and will NOT block rebooting. query by includeLabel means,
// if the query finds an alert, it will include it to the block-list, and it WILL block rebooting.
func (pb PrometheusBlockingChecker) ActiveAlerts() ([]string, error) {
	if pb.initErr != nil {
		return nil, pb.initErr
	}

	api := v1.NewAPI(pb.promClient)

	// run the query against prometheus
	value, _, _, err := api.Query(context.Background(), pb.query, time.Now())
	if err != nil {
		return nil, err
	}

	if value.Type() == model.ValVector {
		if vector, ok := value.(model.Vector); ok {
			activeAlertSet := make(map[string]bool)
			for _, sample := range vector {
				if sample.Value == 0 {
					continue
				}
				// Samples of a custom query do not necessarily carry an alert name:
				// identify them by their labels instead.
				name := sample.Metric.String()
				if alertName, isAlert := sample.Metric[model.AlertNameLabel]; isAlert {
					name = string(alertName)
				}
				if matchesRegex(pb.filter, name, pb.filterMatchOnly) && (!pb.firingOnly || sample.Metric["alertstate"] == "firing") {
					activeAlertSet[name] = true
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
