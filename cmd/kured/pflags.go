package main

import (
	"regexp"
)

type regexpValue struct {
	value **regexp.Regexp
}

func (rev *regexpValue) String() string {
	if *rev.value == nil {
		return ""
	}
	return (*rev.value).String()
}

func (rev *regexpValue) Set(s string) error {
	value, err := regexp.Compile(s)
	if err != nil {
		return err
	}

	*rev.value = value

	return nil
}

func (rev *regexpValue) Type() string {
	return "regexp.Regexp"
}
