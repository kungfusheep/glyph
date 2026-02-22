package forme

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidateOn controls when validation runs. Combine with bitwise OR.
type ValidateOn uint8

const (
	VOnChange ValidateOn = 1 << iota // validate on every keystroke
	VOnBlur                          // validate when field loses focus
	VOnSubmit                        // validate on form submit
)

// StringValidator validates a string value.
type StringValidator func(string) error

// BoolValidator validates a boolean value.
type BoolValidator func(bool) error

// VRequired rejects empty strings.
func VRequired(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("required")
	}
	return nil
}

// VEmail rejects strings that don't look like email addresses.
func VEmail(s string) error {
	if !strings.Contains(s, "@") || !strings.Contains(s, ".") {
		return fmt.Errorf("invalid email")
	}
	at := strings.LastIndex(s, "@")
	if at == 0 || at == len(s)-1 {
		return fmt.Errorf("invalid email")
	}
	domain := s[at+1:]
	if !strings.Contains(domain, ".") || strings.HasSuffix(domain, ".") {
		return fmt.Errorf("invalid email")
	}
	return nil
}

// VMinLen rejects strings shorter than n.
func VMinLen(n int) StringValidator {
	return func(s string) error {
		if len(s) < n {
			return fmt.Errorf("min %d characters", n)
		}
		return nil
	}
}

// VMaxLen rejects strings longer than n.
func VMaxLen(n int) StringValidator {
	return func(s string) error {
		if len(s) > n {
			return fmt.Errorf("max %d characters", n)
		}
		return nil
	}
}

// VMatch rejects strings that don't match the given regex pattern.
func VMatch(pattern string) StringValidator {
	re := regexp.MustCompile(pattern)
	return func(s string) error {
		if s == "" {
			return nil
		}
		if !re.MatchString(s) {
			return fmt.Errorf("invalid format")
		}
		return nil
	}
}

// VTrue rejects false values.
func VTrue(b bool) error {
	if !b {
		return fmt.Errorf("required")
	}
	return nil
}
