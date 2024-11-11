package notifications

import (
	"reflect"
	"testing"
)

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
			name:     "string with length of two is stripped",
			input:    "\"\"",
			expected: "",
		},
		{
			name:     "string with length of two is stripped",
			input:    "''",
			expected: "",
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

func TestNewNotifier(t *testing.T) {
	type args struct {
		URLs []string
	}
	tests := []struct {
		name string
		args args
		want Notifier
	}{
		{
			name: "No URLs means no notifier",
			args: args{},
			want: &NoopNotifier{},
		},
		{
			name: "Empty slice means no notifier",
			args: args{URLs: []string{}},
			want: &NoopNotifier{},
		},
		{
			name: "Empty string means no notifier",
			args: args{URLs: []string{""}},
			want: &NoopNotifier{},
		},
		{
			name: "Pseudo-Empty string means no notifier",
			args: args{URLs: []string{"''"}},
			want: &NoopNotifier{},
		},
		{
			name: "Pseudo-Empty string means no notifier",
			args: args{URLs: []string{"\"\""}},
			want: &NoopNotifier{},
		},
		{
			name: "Invalid string means no notifier",
			args: args{URLs: []string{"'"}},
			want: &NoopNotifier{},
		},
		{
			name: "Old shoutrrr slack urls are not valid anymore",
			args: args{URLs: []string{"slack://xxxx/yyyy/zzzz"}},
			want: &NoopNotifier{},
		},
		{
			name: "Valid slack bot API notifier url",
			args: args{URLs: []string{"slack://xoxb:123456789012-1234567890123-4mt0t4l1YL3g1T5L4cK70k3N@C001CH4NN3L?color=good&title=Great+News&icon=man-scientist&botname=Shoutrrrbot"}},
			want: &ShoutrrrNotifier{},
		},
		{
			name: "Valid slack webhook notifier url",
			args: args{URLs: []string{"slack://hook:WNA3PBYV6-F20DUQND3RQ-Webc4MAvoacrpPakR8phF0zi@webhook?color=good&title=Great+News&icon=man-scientist&botname=Shoutrrrbot"}},
			want: &ShoutrrrNotifier{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewNotifier(tt.args.URLs...)
			if reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("NewNotifier() = %v, want %v", reflect.TypeOf(got), reflect.TypeOf(tt.want))
			}
		})
	}
}
