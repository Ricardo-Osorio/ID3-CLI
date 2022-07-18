package main

import "strings"

func SanitizeInput(input string) string {
	return strings.ReplaceAll(input, "’", "'")
}
