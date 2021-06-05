package alerts

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"time"

	log "github.com/sirupsen/logrus"

	papi "github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"k8s.io/apimachinery/pkg/api/errors"
)

// PromClient is a wrapper around the Prometheus Client interface and implements the api
// This way, the PromClient can be instantiated with the configuration the Client needs, and
// the ability to use the methods the api has, like Query and so on.
type PromClient struct {
	papi papi.Client
	api  v1.API
}

// New creates a new client to the Prometheus API.
// It returns an error on any problem.
func (p PromClient) New(conf papi.Config) (*PromClient, error) {

	promClient, err := papi.NewClient(conf)
	if err != nil {
		return nil, errors.NewServiceUnavailable(err.Error())
	}
	client := PromClient{papi: promClient, api: v1.NewAPI(promClient)}
	return &client, nil
}

// ActiveAlerts is a method of type PromClient, it returns a list of names of active alerts
// (e.g. pending or firing), filtered by the supplied regexp or by the includeLabels query.
// filter by regexp means when the regex finds the alert-name; the alert is exluded from the
// block-list and will NOT block rebooting. query by includeLabel means,
// if the query finds an alert, it will include it to the block-list and it WILL block rebooting.
func (p *PromClient) ActiveAlerts(filter *regexp.Regexp, inclLabel map[string]string) ([]string, error) {
	// var key, val string

	// get all alerts from prometheus
	value, _, err := p.api.Query(context.Background(), "ALERTS", time.Now())
	if err != nil {
		return nil, err
	}

	// TODO: create seperate functions to make it more readable / understand.
	if value.Type() == model.ValVector {
		if vector, ok := value.(model.Vector); ok {
			activeAlertSet := make(map[string]bool)
			for _, sample := range vector {
				if alertName, isAlert := sample.Metric[model.AlertNameLabel]; isAlert && sample.Value != 0 {
					if filter == nil || !filter.MatchString(string(alertName)) {
						log.Info("Added to the BlockingList, by regexFilter: ", alertName)
						activeAlertSet[string(alertName)] = true
					}
					for k, v := range inclLabel {
						if sample.Metric[model.LabelName(k)] == model.LabelValue(v) {
							// because filtering by labels doesn't check for alertname, we want to make sure
							// there's no deadlock, that is, Kured is alerting itself on the block-list.
							if alertName == "RebootRequired" {
								continue
							}
							activeAlertSet[string(alertName)] = true
							log.Info("Added to the BlockingList, by includeLabel: ", alertName)
						}
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

	return nil, fmt.Errorf("Unexpected value type: %v", value)
}
