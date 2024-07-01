package alertmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"
)

const (
	alertManagerPathPrefix = "/api/v2"
	// the default context timeout for alert manager client
	// feel free to change this value/set a corresponding env var if needed
	defaultTimeOut = 10 * time.Second
)

// New is a constructor of AlertManagerClient
//
// if no url flag is given => error
func New(alertManagerURL, alertManagerToken string) (*Client, error) {
	if alertManagerURL == "" {
		return nil, fmt.Errorf("no alert manager url found")
	}
	return &Client{
		Token:   alertManagerToken,
		HostURL: alertManagerURL,
		Client:  new(http.Client),
	}, nil
}

// Status builds the Status endpoint
func (c *Client) Status() *StatusEndpoint {
	return &StatusEndpoint{
		Client: *c,
	}
}

// Silences builds the Silences endpoint
func (c *Client) Silences() *SilencesEndpoint {
	return &SilencesEndpoint{
		Client: *c,
	}
}

// BuildURL builds the full URL for Status Endpoint
func (s *StatusEndpoint) BuildURL() error {
	url, err := url.Parse(s.HostURL)
	if err != nil {
		return err
	}
	url.Path = path.Join(alertManagerPathPrefix, "status")
	s.FullURL = url.String()
	return nil
}

// Get receives information about alert manager overall status
func (s *StatusEndpoint) Get() (*StatusResponse, error) {
	err := s.BuildURL()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeOut)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.FullURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Add("Authentication", fmt.Sprintf("Bearer %s", s.Token))
	response, err := s.Client.Client.Do(request)
	if err != nil {
		return nil, err
	}
	responseObject := new(StatusResponse)
	err = json.NewDecoder(response.Body).Decode(responseObject)
	if err != nil {
		return nil, err
	}
	return responseObject, nil
}

// BuildURL builds the full URL for silences Endpoint
func (s *SilencesEndpoint) BuildURL() error {
	url, err := url.Parse(s.HostURL)
	if err != nil {
		return err
	}
	url.Path = path.Join(alertManagerPathPrefix, "silences")
	s.FullURL = url.String()
	return nil
}

// Get lists the silences
func (s *SilencesEndpoint) Get() ([]GettableSilence, error) {
	err := s.BuildURL()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeOut)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.FullURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Add("Authentication", fmt.Sprintf("Bearer %s", s.Token))
	response, err := s.Client.Client.Do(request)
	if err != nil {
		return nil, err
	}
	responseObject := make([]GettableSilence, 0)
	err = json.NewDecoder(response.Body).Decode(&responseObject)
	if err != nil {
		return nil, err
	}
	if err := ValidateStatus(responseObject); err != nil {
		return nil, err
	}
	return responseObject, nil
}
