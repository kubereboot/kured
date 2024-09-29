package main

import (
	"github.com/spf13/cobra"
	"reflect"
	"testing"
)

func Test_flagCheck(t *testing.T) {
	var cmd *cobra.Command
	var args []string
	slackHookURL = "https://hooks.slack.com/services/BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET"
	expected := "slack://BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET"
	flagCheck(cmd, args)
	if notifyURL != expected {
		t.Errorf("Slack URL Parsing is wrong: expecting %s  but got %s\n", expected, notifyURL)
	}

	// validate that surrounding quotes are stripped
	slackHookURL = "\"https://hooks.slack.com/services/BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET\""
	expected = "slack://BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET"
	flagCheck(cmd, args)
	if notifyURL != expected {
		t.Errorf("Slack URL Parsing is wrong: expecting %s  but got %s\n", expected, notifyURL)
	}
	slackHookURL = "'https://hooks.slack.com/services/BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET'"
	expected = "slack://BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET"
	flagCheck(cmd, args)
	if notifyURL != expected {
		t.Errorf("Slack URL Parsing is wrong: expecting %s  but got %s\n", expected, notifyURL)
	}
	slackHookURL = ""
	notifyURL = "\"teams://79b4XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX@acd8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/204cXXXXXXXXXXXXXXXXXXXXXXXXXXXX/a1f8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX?host=XXXX.webhook.office.com\""
	expected = "teams://79b4XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX@acd8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/204cXXXXXXXXXXXXXXXXXXXXXXXXXXXX/a1f8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX?host=XXXX.webhook.office.com"
	flagCheck(cmd, args)
	if notifyURL != expected {
		t.Errorf("notifyURL Parsing is wrong: expecting %s  but got %s\n", expected, notifyURL)
	}
	notifyURL = "'teams://79b4XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX@acd8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/204cXXXXXXXXXXXXXXXXXXXXXXXXXXXX/a1f8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX?host=XXXX.webhook.office.com'"
	expected = "teams://79b4XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX@acd8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/204cXXXXXXXXXXXXXXXXXXXXXXXXXXXX/a1f8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX?host=XXXX.webhook.office.com"
	flagCheck(cmd, args)
	if notifyURL != expected {
		t.Errorf("notifyURL Parsing is wrong: expecting %s  but got %s\n", expected, notifyURL)
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
