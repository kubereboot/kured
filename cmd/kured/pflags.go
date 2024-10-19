package main

import (
	"regexp"
)

type regexpValue struct {
	*regexp.Regexp
}

func (rev *regexpValue) String() string {
	if rev.Regexp == nil {
		return ""
	}
	return rev.Regexp.String()
}

func (rev *regexpValue) Set(s string) error {
	value, err := regexp.Compile(s)
	if err != nil {
		return err
	}
	rev.Regexp = value
	return nil
}

// Type method returns the type of the flag as a string
func (rev *regexpValue) Type() string {
	return "regexp"
}
