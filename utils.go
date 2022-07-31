package main

import "strings"

// SanitizeInput replaces chars that would otherwise break the
// promptui lib as it renders using golang template package and
// not every char found in the metadata is html friendly
func SanitizeInput(input string) string {
	return strings.ReplaceAll(input, "’", "'")
}

func IsArtistInList(artistID string, list []Artist) bool {
	for _, artist := range list {
		if artistID == artist.ID {
			return true
		}
	}
	return false
}
