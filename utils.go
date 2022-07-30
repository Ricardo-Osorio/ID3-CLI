package main

import "strings"

// SanitizeInput replaces some chars that would otherwise
// break the promptui lib as it renders using golang template
// package and not every char in the metadata is html friendly
func SanitizeInput(input string) string {
	return strings.ReplaceAll(input, "’", "'")
}
