package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/bogem/id3v2"
)

var (
	pathToMusic             string
	autoSelectSingleMatches bool
	renameFiles             bool
	fallbackToFileName      bool
	songLengthDifference    int
)

func ParseFlags() {
	flag.StringVar(&pathToMusic, "path", ".", "Music directory or mp3 file")
	flag.BoolVar(&autoSelectSingleMatches, "auto-handle-single-match", false, "Automatically select a match if it's the only result")
	flag.BoolVar(&renameFiles, "rename-files", false, "Rename mp3 files with artist and song name")
	flag.BoolVar(&fallbackToFileName, "fallback-to-file-name", true, "If there aren't any matches fallback to using the file name for the tags. Expects to find \"<artists> - <song name>\"")
	flag.IntVar(&songLengthDifference, "song-length-difference", 15, "Skip matches if there's this much difference in duration")

	flag.Parse()

	if pathToMusic == "" {
		pathToMusic = "."
	}
}

func main() {
	pathToFpcalc, err := GetFpcalcPath()
	if err != nil {
		fmt.Printf("failed to get fpcalc path: %s\n", err.Error())
		os.Exit(1)
	}
	_, err = os.Open(pathToFpcalc)
	if err != nil {
		fmt.Printf("fpcalc not found\n")
		os.Exit(1)
	}

	ParseFlags()

	if strings.HasSuffix(pathToMusic, ".mp3") {
		// file
		// split and extract the name and full path
		splitPath := strings.Split(pathToMusic, "/")

		fileName := splitPath[len(splitPath)-1]
		pathToMusic = strings.Join(splitPath[:len(splitPath)-1], "/")

		if err = HandleFile(pathToFpcalc, pathToMusic, fileName); err != nil {
			os.Exit(1)
		}
	} else {
		// directory
		dirEntries, err := os.ReadDir(pathToMusic)
		if err != nil {
			fmt.Printf("failed to read directory: %s\n", err.Error())
			os.Exit(1)
		}

		for _, dir := range dirEntries {
			if dir.IsDir() {
				continue
			}
			if !strings.HasSuffix(dir.Name(), ".mp3") {
				continue
			}

			if err = HandleFile(pathToFpcalc, pathToMusic, dir.Name()); err != nil {
				os.Exit(1)
			}
		}
	}
}

func HandleFile(pathToFpcalc, pathToMusic, fileName string) error {
	duration, fingerprint, err := NewFingerprint(pathToFpcalc, pathToMusic+"/"+fileName)
	if err != nil {
		fmt.Printf("failed to generate fingerprint for \"%s\": %s\n", fileName, err.Error())
		return nil
	}

	response, err := Request(duration, fingerprint)
	if err != nil {
		fmt.Printf("failed post request: %s\n", err.Error())
		return err
	}

	tag, err := id3v2.Open(pathToMusic+"/"+fileName, id3v2.Options{Parse: true, ParseFrames: []string{"Artist", "Title", "Album"}})
	if err != nil {
		fmt.Printf("failed to parse mp3 id3 tags: %s\n", err.Error())
		return nil
	}
	defer tag.Close()

	if len(response.Results) == 0 || len(response.Results[0].Recordings) == 0 {
		fmt.Printf("No matches for: %s\n", fileName)

		if fallbackToFileName {
			cleanName := CleanupFileName(fileName)

			// extract artists and song name from the file name
			// expects the format to be "<artists> - <song name>.mp3"
			artist, songName, err := ExtractMetadataFromFileName(cleanName)
			if err != nil {
				fmt.Printf("Failed to extract metadata from file's name: %s\n", err.Error())
				return nil
			}

			err = CommitChangesToFile(tag, artist, songName, "", fileName)
			if err != nil {
				fmt.Printf("Failed to persist changes to file \"%s\": %s\n", fileName, err.Error())
				return err
			}
		}
		return nil
	}

	// Response format
	// List of results, each containing:
	//   - list of songs matched. Each containing:
	//		- list of artists matched
	//   	- list of albums matched
	//   	- title of song
	//   - comparison score (certainty of match)

	input := []MusicMetadata{}
	for _, result := range response.Results {
		music := MusicMetadata{
			Score: result.Score,
		}

		// sort by number of sources
		sort.Slice(result.Recordings, func(i, j int) bool {
			return result.Recordings[i].Sources > result.Recordings[j].Sources
		})

		for _, match := range result.Recordings {
			// exclude empty match
			if len(match.ReleaseGroups) == 0 {
				continue
			}

			if math.Abs(float64(match.Duration-duration)) > float64(songLengthDifference) {
				continue
			}

			music.Sources = match.Sources

			music.Artist = ""
			for _, artist := range match.Artists {
				music.Artist = music.Artist + artist.Name
				if artist.JoinPhrase != "" {
					music.Artist = music.Artist + artist.JoinPhrase
				}
			}
			music.SongName = match.Title

			// TODO
			// this is not just albums, also contains entries of type "single" which should be handled
			for _, albums := range match.ReleaseGroups {
				// skip compilations
				if len(albums.SecondaryTypes) != 0 && albums.SecondaryTypes[0] == "Compilation" {
					continue
				}

				// album matched doesn't include the artist in it
				if len(albums.Artists) != 0 && !IsArtistInList(match.Artists[0].ID, albums.Artists) {
					continue
				}

				music.Album = albums.Title

				input = append(input, music.Copy())
			}
		}
	}

	if len(input) == 0 {
		// TODO
		// fallback to using the file name to populate the tags
		// extracting the artist and song based on a specific format like
		// <artist> - <song name>.mp3
		return nil
	}

	if len(input) == 1 && autoSelectSingleMatches {
		fmt.Printf("Auto selected single match: for %s\n", fileName)
		err = CommitChangesToFile(tag, input[0].Artist, input[0].SongName, input[0].Album, fileName)
		if err != nil {
			fmt.Printf("Failed to persist changes to file \"%s\": %s\n", fileName, err.Error())
			return err
		}
		return nil
	}

	index, err := PromptSelectMatch(fileName, input)
	if err != nil {
		fmt.Printf("failed to run prompt: %s\n", err.Error())
		return nil
	}

	input[index].Artist = SanitizeInput(input[index].Artist)
	input[index].SongName = SanitizeInput(input[index].SongName)
	input[index].Album = SanitizeInput(input[index].Album)

	// build input
	inputTags := []MusicTags{
		MusicTags{Tag: "Save"},
		MusicTags{Tag: "Artist", NewValue: input[index].Artist, OldValue: tag.Artist()},
		MusicTags{Tag: "Song name", NewValue: input[index].SongName, OldValue: tag.Title()},
		MusicTags{Tag: "Album", NewValue: input[index].Album, OldValue: tag.Album()},
	}

	// Repeat until the user has had the opportunity to edit all tags
	// Only "Save" will exit the loop
	for {
		index, err = PromptSelectTag(fileName, inputTags)
		if err != nil {
			break
		}

		// Save options
		if index == 0 {
			err = CommitChangesToFile(tag, inputTags[1].NewValue, inputTags[2].NewValue, inputTags[3].NewValue, fileName)
			if err != nil {
				fmt.Printf("Failed to persist changes to file \"%s\": %s\n", fileName, err.Error())
				break
			}
		}

		newVal, err := PromptNewValue(inputTags[index].NewValue)
		if err != nil {
			break
		}

		inputTags[index].NewValue = newVal
	}
	if err != nil {
		fmt.Printf("failed handling user input: %s\n", err.Error())
		return err
	}
	return nil
}
