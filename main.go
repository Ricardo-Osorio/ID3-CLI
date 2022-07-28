package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/bogem/id3v2"
)

func ParseFlags() string {
	pathToMusic := ""
	flag.StringVar(&pathToMusic, "path", "./", "Music directory or mp3 file")
	flag.Parse()
	return pathToMusic
}

func main() {
	pathToFpcalc := GetFpcalcPath()
	pathToMusic := ParseFlags()

	if pathToMusic == "" {
		pathToMusic = "./"
	}

	if strings.HasSuffix(pathToMusic, ".mp3") {
		// file
		// extract name and path
		path := strings.Split(pathToMusic, "/")
		fileName := path[len(path)-1]
		pathToMusic := strings.ReplaceAll(pathToMusic, "/"+fileName, "")
		HandleFile(pathToFpcalc, pathToMusic, fileName)
	} else {
		// directory
		dirEntries, err := os.ReadDir(pathToMusic)
		if err != nil {
			fmt.Printf("failed to read directory: %s", err.Error())
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

	// response format
	// list of results, each containing:
	//   - list of song matched
	//   - comparison score (certainty of match)
	//
	// each song match contains:
	//   - list of artists matched
	//   - list of albums matched
	//   - title of song
	// creating many different possibilities from a single match :sigh:

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
			// due to unknow reasons a song match can be a single ID
			// and everything else empty values (empty string, emtpy lists, etc)
			if len(match.ReleaseGroups) == 0 {
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
			// use the field "duration" and check if it's too different

			// TODO
			// MusicBrainz Picard also has tags for Date. Hows does it get it?

			// TODO
			// this is not just albums, also contains entries of type "single"
			// this also contains a list of artists
			// the latter could be useful to cross match with what's in result.Recordings[?].Artists[:]
			//   I.E. if song is matched to two artists and the album is matched to a single one or different names?
			for _, albums := range match.ReleaseGroups {
				// skip compilations (kind of personal preference)
				if len(albums.SecondaryTypes) != 0 && albums.SecondaryTypes[0] == "Compilation" {
					continue
				}

				music.Album = albums.Title

				input = append(input, music.Copy())
			}
		}
	}

	if len(input) == 0 {
		return nil
	}

	index, err := PromptSelectMatch(fileName, input)
	if err != nil {
		// TODO add log message or customize error
		return nil
	}

	tag, err := id3v2.Open(pathToMusic+"/"+fileName, id3v2.Options{Parse: true, ParseFrames: []string{"Artist", "Title", "Album"}})
	if err != nil {
		fmt.Printf("failed to parse mp3 id3 tags: %s", err.Error())
		return nil
	}
	defer tag.Close()

	// build input
	inputTags := []MusicTags{
		MusicTags{Tag: "Save"},
		MusicTags{Tag: "Artist", NewValue: input[index].Artist, OldValue: tag.Artist()},
		MusicTags{Tag: "Song name", NewValue: input[index].SongName, OldValue: tag.Title()},
		MusicTags{Tag: "Album", NewValue: input[index].Album, OldValue: tag.Album()},
	}

	// Repeat until the user has had the opportunity to edit all tags
	// Only "continue" will exit the loop
	for {
		index, err = PromptSelectTag(fileName, inputTags)
		if err != nil {
			break
		}

		if index == 0 {
			// TODO sanitize input earlier so it doesn't save a value
			// different than what is show to the user
			tag.SetArtist(SanitizeInput(inputTags[1].NewValue))
			tag.SetTitle(SanitizeInput(inputTags[2].NewValue))
			tag.SetAlbum(SanitizeInput(inputTags[3].NewValue))
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
