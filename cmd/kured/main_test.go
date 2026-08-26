package main

import (
	"net/url"
	"reflect"
	"testing"

	"github.com/nicholas-fedor/shoutrrr"
)

func TestValidateNotificationURL(t *testing.T) {
	const teamsNotifyURL = "teams://?host=https%3A%2F%2Fprod-00.westus.logic.azure.com%3A443%2Fworkflows%2Fabc123%2Ftriggers%2Fmanual%2Fpaths%2Finvoke%3Fapi-version%3D2016-06-00%26sp%3D%2Ftriggers%2Fmanual%2Frun%26sv%3D1.0%26sig%3DXXXXXXXX"

	tests := []struct {
		name         string
		slackHookURL string
		notifyURL    string
		expected     string
	}{
		{"slackHookURL only works fine", "https://hooks.slack.com/services/BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET", "", "slack://BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET"},
		{"slackHookURL and notify URL together only keeps notifyURL", "\"https://hooks.slack.com/services/BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET\"", teamsNotifyURL, teamsNotifyURL},
		{"slackHookURL removes extraneous double quotes", "\"https://hooks.slack.com/services/BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET\"", "", "slack://BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET"},
		{"slackHookURL removes extraneous single quotes", "'https://hooks.slack.com/services/BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET'", "", "slack://BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET"},
		{"notifyURL removes extraneous double quotes", "", "\"" + teamsNotifyURL + "\"", teamsNotifyURL},
		{"notifyURL removes extraneous single quotes", "", "'" + teamsNotifyURL + "'", teamsNotifyURL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateNotificationURL(tt.notifyURL, tt.slackHookURL); !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("validateNotificationURL() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestTeamsPowerAutomateNotificationURLParses(t *testing.T) {
	workflowURL := "https://prod-00.westus.logic.azure.com:443/workflows/abc123/triggers/manual/paths/invoke?api-version=2016-06-00&sp=/triggers/manual/run&sv=1.0&sig=XXXXXXXX"
	notifyURL := "teams://?host=" + url.QueryEscape(workflowURL)

	if _, err := shoutrrr.CreateSender(notifyURL); err != nil {
		t.Fatalf("CreateSender(%q) returned error: %v", notifyURL, err)
	}
}

func Test_stripQuotes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "string with no surrounding quotes is unchanged",
			input:    "Hello, world!",
			expected: "Hello, world!",
		},
		{
			name:     "string with surrounding double quotes should strip quotes",
			input:    "\"Hello, world!\"",
			expected: "Hello, world!",
		},
		{
			name:     "string with surrounding single quotes should strip quotes",
			input:    "'Hello, world!'",
			expected: "Hello, world!",
		},
		{
			name:     "string with unbalanced surrounding quotes is unchanged",
			input:    "'Hello, world!\"",
			expected: "'Hello, world!\"",
		},
		{
			name:     "string with length of one is unchanged",
			input:    "'",
			expected: "'",
		},
		{
			name:     "string with length of zero is unchanged",
			input:    "",
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripQuotes(tt.input); !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("stripQuotes() = %v, expected %v", got, tt.expected)
			}
		})
	}
}
