package main

import (
	"fmt"
	"regexp"
)

// Custom flag type to handle *regexp.Regexp with pflag
type regexpValue struct {
	value **regexp.Regexp
}

// String method to return the string representation of the value
func (rev *regexpValue) String() string {
	if *rev.value != nil {
		return (*rev.value).String()
	}
	return ""
}

// Set method to parse the input string and set the regexp value
func (rev *regexpValue) Set(s string) error {
	compiledRegexp, err := regexp.Compile(s)
	if err != nil {
		return fmt.Errorf("invalid regular expression: %w", err)
	}
	*rev.value = compiledRegexp
	return nil
}

// Type method returns the type of the flag as a string
func (rev *regexpValue) Type() string {
	return "regexp"
}
