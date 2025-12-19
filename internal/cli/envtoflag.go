// Package cli contains tools for command line parsing in the different daemons.
package cli

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	flag "github.com/spf13/pflag"
)

const (
	// EnvPrefix The environment variable prefix of all environment variables bound to our command line flags.
	EnvPrefix = "KURED"
)

// RegexpValue is a flag.Value that stores a regexp and allow quick input validation
type RegexpValue struct {
	*regexp.Regexp
}

// String method returns the string representation of the regexp.
// It was necessary to override the default String method to avoid a panic
func (rev *RegexpValue) String() string {
	if rev.Regexp == nil {
		return ""
	}
	return rev.Regexp.String()
}

// Set method sets the regexp from a string. // Autofilled by my IDE. Great.
func (rev *RegexpValue) Set(s string) error {
	value, err := regexp.Compile(s)
	if err != nil {
		return err
	}
	rev.Regexp = value
	return nil
}

// Type method returns the type of the flag as a string
func (rev *RegexpValue) Type() string {
	return "regexp"
}

// LoadFromEnv attempts to load environment variables corresponding to flags.
// It looks for an environment variable with the uppercase version of the flag name (prefixed by EnvPrefix).
func LoadFromEnv() {
	flag.VisitAll(func(f *flag.Flag) {
		envVarName := fmt.Sprintf("%s_%s", EnvPrefix, strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_")))

		if envValue, exists := os.LookupEnv(envVarName); exists {
			switch f.Value.Type() {
			case "int":
				if parsedVal, err := strconv.Atoi(envValue); err == nil {
					err := flag.Set(f.Name, strconv.Itoa(parsedVal))
					if err != nil {
						fmt.Printf("cannot set flag %s from env var named %s", f.Name, envVarName)
						os.Exit(1)
					} // Set int flag
				} else {
					fmt.Printf("Invalid value for env var named %s", envVarName)
					os.Exit(1)
				}
			case "string":
				err := flag.Set(f.Name, envValue)
				if err != nil {
					fmt.Printf("cannot set flag %s from env{%s}: %s\n", f.Name, envVarName, envValue)
					os.Exit(1)
				} // Set string flag
			case "bool":
				if parsedVal, err := strconv.ParseBool(envValue); err == nil {
					err := flag.Set(f.Name, strconv.FormatBool(parsedVal))
					if err != nil {
						fmt.Printf("cannot set flag %s from env{%s}: %s\n", f.Name, envVarName, envValue)
						os.Exit(1)
					} // Set boolean flag
				} else {
					fmt.Printf("Invalid value for %s: %s\n", envVarName, envValue)
					os.Exit(1)
				}
			case "duration":
				// Set duration from the environment variable (e.g., "1h30m")
				if _, err := time.ParseDuration(envValue); err == nil {
					err = flag.Set(f.Name, envValue)
					if err != nil {
						fmt.Printf("cannot set flag %s from env{%s}: %s\n", f.Name, envVarName, envValue)
						os.Exit(1)
					}
				} else {
					fmt.Printf("Invalid duration for %s: %s\n", envVarName, envValue)
					os.Exit(1)
				}
			case "regexp":
				// For regexp, set it from the environment variable
				err := flag.Set(f.Name, envValue)
				if err != nil {
					fmt.Printf("cannot set flag %s from env{%s}: %s\n", f.Name, envVarName, envValue)
					os.Exit(1)
				}
			case "stringSlice":
				// For stringSlice, split the environment variable by commas and set it
				err := flag.Set(f.Name, envValue)
				if err != nil {
					fmt.Printf("cannot set flag %s from env{%s}: %s\n", f.Name, envVarName, envValue)
					os.Exit(1)
				}
			default:
				// String arrays are not supported from CLI
				fmt.Printf("Unsupported flag type for %s\n", f.Name)
			}
		}
	})

}
