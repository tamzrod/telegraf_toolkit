package metadatastore

import "strings"

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// isUniqueViolation reports whether an error is a SQLite uniqueness failure.
// The modernc driver reports it in the message rather than a typed code, so the
// message is inspected; the check is deliberately narrow.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToUpper(err.Error())
	return strings.Contains(msg, "UNIQUE CONSTRAINT FAILED") || strings.Contains(msg, "CONSTRAINT FAILED: UNIQUE")
}
