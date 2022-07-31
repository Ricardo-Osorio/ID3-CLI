package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/bogem/id3v2"
)

var (
	pathToMusic             string
	autoSelectSingleMatches bool
	songLengthDifference    int
)

func ParseFlags() {
	flag.StringVar(&pathToMusic, "path", ".", "Music directory or mp3 file")
	flag.BoolVar(&autoSelectSingleMatches, "auto-handle-single-match", false, "Automatically select a match if it's the only result")
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
		// split and extract the name and path
		splitPath := strings.Split(pathToMusic, "/")
		fileName := splitPath[len(splitPath)-1]
		pathToMusic = strings.ReplaceAll(pathToMusic, "/"+fileName, "")

		HandleFile(pathToFpcalc, pathToMusic, fileName)
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

			HandleFile(pathToFpcalc, pathToMusic, dir.Name())
		}
	}
}

func HandleFile(pathToFpcalc, pathToMusic, fileName string) error {
	duration, fingerprint, err := NewFingerprint(pathToFpcalc, pathToMusic+"/"+fileName)
	if err != nil {
		fmt.Printf("failed to generate fingerprint: %s\n", err.Error())
		return nil
	}

	response, err := Request(duration, fingerprint)
	if err != nil {
		fmt.Printf("failed post request: %s\n", err.Error())
		return err
	}

	// TODO: use the file as "name - song.mp3" for the tags
	if len(response.Results) == 0 || len(response.Results[0].Recordings) == 0 {
		fmt.Printf("No matches for: %s\n", fileName)
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
			// MusicBrainz's Picard also has tags for Date. How does it get it?

			// TODO
			// this is not just albums, also contains entries of type "single"
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
		// TODO use file name to populate the tags
		return nil
	}

	tag, err := id3v2.Open(pathToMusic+"/"+fileName, id3v2.Options{Parse: true, ParseFrames: []string{"Artist", "Title", "Album"}})
	if err != nil {
		fmt.Printf("failed to parse mp3 id3 tags: %s\n", err.Error())
		return nil
	}
	defer tag.Close()

	if len(input) == 1 && autoSelectSingleMatches {
		tag.SetArtist(input[0].Artist)
		tag.SetTitle(input[0].SongName)
		tag.SetAlbum(input[0].Album)
		// persist new tags
		if err = tag.Save(); err != nil {
			fmt.Printf("failed to store tags: %s\n", err.Error())
		}
		fmt.Printf("Auto selected single match: for %s\n", fileName)
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

		if index == 0 {
			tag.SetArtist(inputTags[1].NewValue)
			tag.SetTitle(inputTags[2].NewValue)
			tag.SetAlbum(inputTags[3].NewValue)
			// persist new tags
			if err = tag.Save(); err != nil {
				fmt.Printf("failed to store tags: %s\n", err.Error())
			}
			break
		}

		newVal, err := PromptNewValue(inputTags[index].NewValue)
		if err != nil {
			break
		}

		inputTags[index].NewValue = newVal
	}
	if err != nil {
		fmt.Printf("failed handling user input: %s\n", err.Error())
		return nil
	}
	return nil
}

// NewFingerprint runs fpcalc against a file to generate a acoustID
// Returns the duration of the song (s) and fingerprint
func NewFingerprint(fpcalcPath, file string) (int, string, error) {
	out, err := exec.Command(fpcalcPath, "-json", file).Output()
	if err != nil {
		return 0, "", err
	}

	var output struct {
		Duration    float64 `json:"duration"`
		Fingerprint string  `json:"fingerprint"`
	}

	err = json.Unmarshal(out, &output)
	if err != nil {
		return 0, "", fmt.Errorf("invalid JSON output from fpcalc: %w", err)
	}

	return int(output.Duration), output.Fingerprint, nil
}
