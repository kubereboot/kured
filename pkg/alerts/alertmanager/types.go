package alertmanager

import (
	"fmt"
	"net/http"
)

var (
	silenceStates = map[string]bool{"expired": true, "active": true, "pending": true}
)

// Client is the object of the alert manager client
type Client struct {
	Token   string       `json:"token" yaml:"token"`
	HostURL string       `json:"hostUrl" yaml:"hostUrl"`
	Client  *http.Client `json:"client" yaml:"client"`
}

// StatusEndpoint is the status enpoint of the alert manager client
type StatusEndpoint struct {
	Client  `json:"alertmanagerClient" yaml:"alertmanagerClient"`
	FullURL string `json:"fullUrl" yaml:"fullUrl"`
}

// SilencesEndpoint is the silences enpoint of the alert manager client
type SilencesEndpoint struct {
	Client  `json:"alertmanagerClient" yaml:"alertmanagerClient"`
	FullURL string `json:"fullUrl" yaml:"fullUrl"`
}

// StatusResponse is the object returned when sending GET $(host_url)$(path_prefix)/status request
type StatusResponse struct {
	Cluster     ClusterStatus `json:"cluster" yaml:"cluster"`
	VersionInfo VersionInfo   `json:"versionInfo" yaml:"versionInfo"`
	Config      Config        `json:"alertmanagerConfig" yaml:"alertmanagerConfig"`
	Uptime      string        `json:"uptime" yaml:"uptime"`
}

// ClusterStatus is the status of the cluster
type ClusterStatus struct {
	Name   string       `json:"name" yaml:"name"`
	Status string       `json:"status" yaml:"status"`
	Peers  []PeerStatus `json:"peers" yaml:"peers"`
}

// PeerStatus is part of get status response
type PeerStatus struct {
	Name    string `json:"name" yaml:"name"`
	Address string `json:"address" yaml:"address"`
}

// VersionInfo contains various go and alert manager version info
type VersionInfo struct {
	Version   string `json:"version" yaml:"version"`
	Revision  string `json:"revision" yaml:"revision"`
	Branch    string `json:"branch" yaml:"branch"`
	BuildUser string `json:"buildUser" yaml:"buildUser"`
	BuildData string `json:"buildData" yaml:"buildData"`
	GoVersion string `json:"goVersion" yaml:"goVersion"`
}

// Config contains a string
type Config struct {
	Original string `json:"original" yaml:"original"`
}

// GettableSilence is the response when sending GET $(host_url)$(path_prefix)/silences request
type GettableSilence struct {
	ID        string        `json:"id" yaml:"id"`
	Status    SilenceStatus `json:"status" yaml:"status"`
	UpdatedAt string        `json:"updatedAt" yaml:"updatedAt"`
}

// SilenceStatus shows the state of the silence
type SilenceStatus struct {
	State string `json:"state" yaml:"state"`
}

// Validate is validating if the status string corresponds to any of the pre-defined dict elements
func (s SilenceStatus) Validate() error {
	if !silenceStates[s.State] {
		return fmt.Errorf("such silence state does not exist: %s", s.State)
	}
	return nil
}

// ValidateStatus is checking the whole slice of GettableSilences if silence.status has the right values
func ValidateStatus(g []GettableSilence) error {
	for _, silence := range g {
		if err := silence.Status.Validate(); err != nil {
			return err
		}
	}
	return nil
}
