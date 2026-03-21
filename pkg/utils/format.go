package utils

import "strconv"

// Int64ToString converts an int64 to its decimal string representation.
func Int64ToString(n int64) string {
	return strconv.FormatInt(n, 10)
}

// ParseUUID parses a string UUID, returning an error string if invalid.
func ParseUUID(s string) (string, bool) {
	if len(s) != 36 {
		return "", false
	}
	// Quick sanity check for UUID format xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return "", false
			}
		} else if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return "", false
		}
	}
	return s, true
}
