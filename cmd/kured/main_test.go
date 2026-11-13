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

func TestValidateNodeLabels(t *testing.T) {
	// wantErr is the sentinel the returned error should wrap (nil means the
	// input is valid). A malformed label is fatal at startup, a key mismatch is
	// only a warning, so the caller distinguishes them via errors.Is and the
	// test pins down which sentinel each case yields.
	tests := []struct {
		name       string
		preReboot  []string
		postReboot []string
		wantErr    error
	}{
		{
			name:       "matching key=value labels are accepted",
			preReboot:  []string{"maintenance=true"},
			postReboot: []string{"maintenance=false"},
			wantErr:    nil,
		},
		{
			name:       "empty label value is accepted",
			preReboot:  []string{"maintenance="},
			postReboot: []string{"maintenance="},
			wantErr:    nil,
		},
		{
			name:       "label without = is malformed instead of panicking",
			preReboot:  []string{"node.kubernetes.io/exclude-from-external-load-balancers-"},
			postReboot: []string{"node.kubernetes.io/exclude-from-external-load-balancers-"},
			wantErr:    errMalformedNodeLabel,
		},
		{
			name:       "label with empty key is malformed",
			preReboot:  []string{"=true"},
			postReboot: []string{"=true"},
			wantErr:    errMalformedNodeLabel,
		},
		{
			name:       "mismatched keys are reported as a mismatch",
			preReboot:  []string{"maintenance=true"},
			postReboot: []string{"other=false"},
			wantErr:    errMismatchedNodeLabelKeys,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNodeLabels(tt.preReboot, tt.postReboot)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("validateNodeLabels() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("validateNodeLabels() = %v, want error wrapping %v", err, tt.wantErr)
			}
			// A malformed label must not be mistaken for a mere mismatch (which
			// would only warn) and vice versa.
			if tt.wantErr == errMalformedNodeLabel && errors.Is(err, errMismatchedNodeLabelKeys) {
				t.Errorf("validateNodeLabels() = %v, malformed label must not wrap errMismatchedNodeLabelKeys", err)
			}
			if tt.wantErr == errMismatchedNodeLabelKeys && errors.Is(err, errMalformedNodeLabel) {
				t.Errorf("validateNodeLabels() = %v, key mismatch must not wrap errMalformedNodeLabel", err)
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
