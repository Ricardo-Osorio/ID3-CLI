package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// SanitizeInput replaces chars that would otherwise break the
// promptui lib as it renders using golang template package and
// not every char found in the metadata is html friendly
func SanitizeInput(input string) string {
	input = strings.ReplaceAll(input, "→", "-")
	return strings.ReplaceAll(input, "’", "'")
}

// CleanupFileName will attempt to remove the most common occurrences
// of filler phrases and words found on most youtube videos for songs
// like:
//  - Official music video
//  - Official audio
//  - Official video
//  - Official
//  - Lyrics video
//  - Lyric video
//  - Lyrics
//  - HQ
//  - HD
//  - (Prod. name)
// And variations of the above with "(", "["
func CleanupFileName(input string) string {
	// TODO hide error
	targets := []string{
		// official, official audio, official video, official music video
		// with all variations of being wrapped in " ", "(" or "["
		"(?i)[([]?official ?(audio|(music )?video)?[)\\]]?",
		// lyrics video, lyric video, lyrics
		// with all variations of being wrapped in " ", "(" or "["
		"(?i)[([]?lyrics?( video)?[)\\]]?",
		// Prod. name
		// with all variations of being wrapped in " ", "(" or "["
		"(?i)[([]?prod.?.*[)\\]]?",
	}
	for _, regex := range targets {
		exp, err := regexp.Compile("")
		if err != nil {
			fmt.Printf("error compiling regex: %s\n", err.Error())
			return ""
		}
		match := exp.FindString(regex)
		if match != "" {
			input = strings.ReplaceAll(input, match, "")
		}
	}
	return input
}

func IsArtistInList(artistID string, list []Artist) bool {
	for _, artist := range list {
		if artistID == artist.ID {
			return true
		}
	}
	return false
}

// ExtractMetadataFromFileName will split and extract both artist and
// song name from a file's name. Expects the name to be in the format
// "<artists> - <song name>.mp3", otherwise returns an error.
func ExtractMetadataFromFileName(filename string) (string, string, error) {
	split := strings.Split(filename, " - ")
	if len(split) != 2 {
		return "", "", errors.New("not in supported format")
	}
	// exclude the suffix ".mp3"
	songName := split[1][:len(split[1])-4]
	return split[0], songName, nil
}
