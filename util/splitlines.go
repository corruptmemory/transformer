package util

import (
	"strings"
)

func SplitLines(s string) []string {
	if len(s) == 0 {
		return nil
	}
	if strings.Contains(s, "\r\n") {
		return strings.Split(s, "\r\n")
	}
	return strings.Split(s, "\n")
}

func RemoveEmptySplits(s []string, trim bool) []string {
	result := make([]string, 0, len(s))
	for _, line := range s {
		if trim {
			line = strings.TrimSpace(line)
		}
		if len(line) > 0 {
			result = append(result, line)
		}
	}
	return result
}
