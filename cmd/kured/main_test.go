package main

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kubereboot/kured/pkg/daemonsetlock"
)

// fakeLock is a minimal daemonsetlock.Lock used to observe call ordering in tests.
type fakeLock struct {
	onRelease func()
}

func (f *fakeLock) Acquire(daemonsetlock.NodeMeta) (bool, string, error) { return true, "", nil }
func (f *fakeLock) Holding() (bool, daemonsetlock.LockAnnotationValue, error) {
	return true, daemonsetlock.LockAnnotationValue{}, nil
}
func (f *fakeLock) Release() error {
	if f.onRelease != nil {
		f.onRelease()
	}
	return nil
}

func TestUncordonThenReleaseUncordonsBeforeReleasingLock(t *testing.T) {
	var order []string

	lock := &fakeLock{onRelease: func() { order = append(order, "release") }}
	uncordonFn := func() error {
		order = append(order, "uncordon")
		return nil
	}

	uncordonThenRelease("test-node", lock, uncordonFn)

	if want := []string{"uncordon", "release"}; !reflect.DeepEqual(order, want) {
		t.Errorf("uncordonThenRelease() call order = %v, want %v", order, want)
	}
}

func TestUncordonThenReleaseReleasesLockEvenIfUncordonFails(t *testing.T) {
	released := false
	lock := &fakeLock{onRelease: func() { released = true }}
	uncordonFn := func() error { return errors.New("boom") }

	uncordonThenRelease("test-node", lock, uncordonFn)

	if !released {
		t.Errorf("uncordonThenRelease() did not release the lock after a failed uncordon")
	}
}

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
