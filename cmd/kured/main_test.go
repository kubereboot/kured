package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestValidateNotificationURL(t *testing.T) {

	tests := []struct {
		name         string
		slackHookURL string
		notifyURL    string
		expected     string
	}{
		{"slackHookURL only works fine", "https://hooks.slack.com/services/BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET", "", "slack://BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET"},
		{"slackHookURL and notify URL together only keeps notifyURL", "\"https://hooks.slack.com/services/BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET\"", "teams://79b4XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX@acd8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/204cXXXXXXXXXXXXXXXXXXXXXXXXXXXX/a1f8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX?host=XXXX.webhook.office.com", "teams://79b4XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX@acd8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/204cXXXXXXXXXXXXXXXXXXXXXXXXXXXX/a1f8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX?host=XXXX.webhook.office.com"},
		{"slackHookURL removes extraneous double quotes", "\"https://hooks.slack.com/services/BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET\"", "", "slack://BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET"},
		{"slackHookURL removes extraneous single quotes", "'https://hooks.slack.com/services/BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET'", "", "slack://BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET"},
		{"notifyURL removes extraneous double quotes", "", "\"teams://79b4XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX@acd8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/204cXXXXXXXXXXXXXXXXXXXXXXXXXXXX/a1f8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX?host=XXXX.webhook.office.com\"", "teams://79b4XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX@acd8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/204cXXXXXXXXXXXXXXXXXXXXXXXXXXXX/a1f8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX?host=XXXX.webhook.office.com"},
		{"notifyURL removes extraneous single quotes", "", "'teams://79b4XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX@acd8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/204cXXXXXXXXXXXXXXXXXXXXXXXXXXXX/a1f8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX?host=XXXX.webhook.office.com'", "teams://79b4XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX@acd8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/204cXXXXXXXXXXXXXXXXXXXXXXXXXXXX/a1f8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX?host=XXXX.webhook.office.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateNotificationURL(tt.notifyURL, tt.slackHookURL); !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("validateNotificationURL() = %v, expected %v", got, tt.expected)
			}
		})
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

func Test_abortFailedDrain(t *testing.T) {
	tests := []struct {
		name        string
		uncordonErr error
		releaseErr  error
	}{
		{name: "uncordons before releasing the lock"},
		{name: "still releases the lock when uncordon fails", uncordonErr: errors.New("uncordon failed")},
		{name: "still uncordons when releasing the lock fails", releaseErr: errors.New("release failed")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			abortFailedDrain("node-1",
				func() error { calls = append(calls, "uncordon"); return tt.uncordonErr },
				func() error { calls = append(calls, "release"); return tt.releaseErr },
			)
			// Releasing the lock waits for --lock-release-delay, so the node has
			// to be uncordoned before it, not after.
			if want := []string{"uncordon", "release"}; !reflect.DeepEqual(calls, want) {
				t.Errorf("abortFailedDrain() calls = %v, expected %v", calls, want)
			}
		})
	}
}
