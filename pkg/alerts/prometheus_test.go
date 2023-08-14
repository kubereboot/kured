package alerts

import (
	"log"
	"net/http"
	"net/http/httptest"

	"regexp"
	"testing"

	"github.com/prometheus/client_golang/api"

	"github.com/stretchr/testify/assert"
)

type MockResponse struct {
	StatusCode int
	Body       []byte
}

// MockServerProperties ties a mock response to a url and a method
type MockServerProperties struct {
	URI        string
	HTTPMethod string
	Response   MockResponse
}

// NewMockServer sets up a new MockServer with properties ad starts the server.
func NewMockServer(props ...MockServerProperties) *httptest.Server {

	handler := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			for _, proc := range props {
				_, err := w.Write(proc.Response.Body)
				if err != nil {
					log.Fatal(err)
				}
			}
		})
	return httptest.NewServer(handler)
}

func TestActiveAlerts(t *testing.T) {
	responsebody := `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"ALERTS","alertname":"GatekeeperViolations","alertstate":"firing","severity":"warning","team":"platform-infra"},"value":[1622472933.973,"1"]},{"metric":{"__name__":"ALERTS","alertname":"PodCrashing-dev","alertstate":"firing","container":"deployment","instance":"1.2.3.4:8080","job":"kube-state-metrics","namespace":"dev","pod":"dev-deployment-78dcbmf25v","severity":"critical","team":"dev"},"value":[1622472933.973,"1"]},{"metric":{"__name__":"ALERTS","alertname":"PodRestart-dev","alertstate":"firing","container":"deployment","instance":"1.2.3.4:1234","job":"kube-state-metrics","namespace":"qa","pod":"qa-job-deployment-78dcbmf25v","severity":"warning","team":"qa"},"value":[1622472933.973,"1"]},{"metric":{"__name__":"ALERTS","alertname":"PrometheusTargetDown","alertstate":"firing","job":"kubernetes-pods","severity":"warning","team":"platform-infra"},"value":[1622472933.973,"1"]},{"metric":{"__name__":"ALERTS","alertname":"ScheduledRebootFailing","alertstate":"pending","severity":"warning","team":"platform-infra"},"value":[1622472933.973,"1"]}]}}`
	addr := "http://localhost:10001"

	for _, tc := range []struct {
		it              string
		rFilter         string
		respBody        string
		aName           string
		wantN           int
		firingOnly      bool
		filterMatchOnly bool
	}{
		{
			it:              "should return no active alerts",
			respBody:        responsebody,
			rFilter:         "",
			wantN:           0,
			firingOnly:      false,
			filterMatchOnly: false,
		},
		{
			it:              "should return a subset of all alerts",
			respBody:        responsebody,
			rFilter:         "Pod",
			wantN:           3,
			firingOnly:      false,
			filterMatchOnly: false,
		},
		{
			it:              "should return a subset of all alerts",
			respBody:        responsebody,
			rFilter:         "Gatekeeper",
			wantN:           1,
			firingOnly:      false,
			filterMatchOnly: true,
		},
		{
			it:              "should return all active alerts by regex",
			respBody:        responsebody,
			rFilter:         "*",
			wantN:           5,
			firingOnly:      false,
			filterMatchOnly: false,
		},
		{
			it:              "should return all active alerts by regex filter",
			respBody:        responsebody,
			rFilter:         "*",
			wantN:           5,
			firingOnly:      false,
			filterMatchOnly: false,
		},
		{
			it:              "should return only firing alerts if firingOnly is true",
			respBody:        responsebody,
			rFilter:         "*",
			wantN:           4,
			firingOnly:      true,
			filterMatchOnly: false,
		},

		{
			it:              "should return ScheduledRebootFailing active alerts",
			respBody:        `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"ALERTS","alertname":"ScheduledRebootFailing","alertstate":"pending","severity":"warning","team":"platform-infra"},"value":[1622472933.973,"1"]}]}}`,
			aName:           "ScheduledRebootFailing",
			rFilter:         "*",
			wantN:           1,
			firingOnly:      false,
			filterMatchOnly: false,
		},
		{
			it:              "should not return an active alert if RebootRequired is firing (regex filter)",
			respBody:        `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"ALERTS","alertname":"RebootRequired","alertstate":"pending","severity":"warning","team":"platform-infra"},"value":[1622472933.973,"1"]}]}}`,
			rFilter:         "RebootRequired",
			wantN:           0,
			firingOnly:      false,
			filterMatchOnly: false,
		},
		{
			it:              "should not return an active alert if RebootRequired is firing (regex filter)",
			respBody:        `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"ALERTS","alertname":"RebootRequired","alertstate":"pending","severity":"warning","team":"platform-infra"},"value":[1622472933.973,"1"]}]}}`,
			rFilter:         "RebootRequired",
			wantN:           1,
			firingOnly:      false,
			filterMatchOnly: true,
		},
	} {
		// Start mockServer
		mockServer := NewMockServer(MockServerProperties{
			URI:        addr,
			HTTPMethod: http.MethodPost,
			Response: MockResponse{
				Body: []byte(tc.respBody),
			},
		})
		// Close mockServer after all connections are gone
		defer mockServer.Close()

		t.Run(tc.it, func(t *testing.T) {

			// regex filter
			regex, _ := regexp.Compile(tc.rFilter)

			// instantiate the prometheus client with the mockserver-address
			p, err := NewPromClient(api.Config{Address: mockServer.URL})
			if err != nil {
				log.Fatal(err)
			}

			result, err := p.ActiveAlerts(regex, tc.firingOnly, tc.filterMatchOnly)
			if err != nil {
				log.Fatal(err)
			}

			// assert
			assert.Equal(t, tc.wantN, len(result), "expected amount of alerts %v, got %v", tc.wantN, len(result))

			if tc.aName != "" {
				assert.Equal(t, tc.aName, result[0], "expected active alert %v, got %v", tc.aName, result[0])
			}
		})
	}
}
