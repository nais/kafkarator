package utils

import (
	"fmt"
	"strings"
)

var triggerWords = []string{
	"password",
	"token",
	"secret",
	"private key",
	"certificate",
	"avns_",
}

// CheckForPossibleCredentials checks if an error message contains things that looks like credentials
// If it does, return a new error with a message that the error message has been removed because it contains possible credentials.
// Else the error is returned unchanged.
func CheckForPossibleCredentials(err error) error {
	lowerErr := strings.ToLower(err.Error())
	for _, trigger := range triggerWords {
		if strings.Contains(lowerErr, trigger) {
			return fmt.Errorf("error message has been removed because it contains possible credentials")
		}
	}
	return err
}
