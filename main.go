package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/bogem/id3v2"
)

func main() {
	pathToFpcalc := GetFpcalcPath()
	pathToDir := ParseFlags()

	if pathToDir == "" {
		var err error
		pathToDir, err = AskForDirectory()
		if err != nil {
			fmt.Printf("failed to read current directly: %s", err.Error())
			os.Exit(1)
		}
	}

	dirEntries, err := os.ReadDir(pathToDir)
	if err != nil {
		fmt.Printf("failed to read directory: %s", err.Error())
		os.Exit(1)
	}

	for _, dir := range dirEntries {
		if dir.IsDir() {
			// fmt.Printf("folder \"%s\" skipped\n", dir.Name())
			continue
		}
		if !strings.HasSuffix(dir.Name(), ".mp3") {
			// fmt.Printf("file \"%s\" ignored due to extension\n", dir.Name())
			continue
		}

		// ## test
		t := dir.Name()
		fmt.Print(t)
		// ##

		duration, fingerprint, err := NewFingerprint(pathToFpcalc, pathToDir+"/"+dir.Name())
		if err != nil {
			fmt.Printf("failed to generate fingerprint: %s\n", err.Error())
			continue
		}

		response, err := Request(duration, fingerprint)
		if err != nil {
			fmt.Printf("failed post request: %s\n", err.Error())
			continue
		}

		if len(response.Results) == 0 || len(response.Results[0].Recordings) == 0 {
			fmt.Println("no matches")
			continue
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
			for _, match := range result.Recordings {
				// due to unknow reasons a song match can be an empty struct
				if len(match.ReleaseGroups) == 0 {
					continue
				}

				music.Artist = ""

				for _, artist := range match.Artists {
					music.Artist = music.Artist + artist.Name
					if artist.JoinPhrase != "" {
						music.Artist = music.Artist + artist.JoinPhrase
					}
				}
				music.SongName = match.Title

				// TODO
				// this is not just albums, also contains entries of type "single"
				// this also contains a list of artists
				// the latter could be useful to cross match with what's in result.Recordings[?].Artists[:]
				//   I.E. if song is matched to two artists and the album is matched to a single one or different names?
				for _, albums := range match.ReleaseGroups {
					// skip compilations (personal preference but could be a flag)
					if len(albums.SecondaryTypes) != 0 && albums.SecondaryTypes[0] == "Compilation" {
						continue
					}

					music.Album = albums.Title

					input = append(input, music.Copy())
				}
			}
		}

		index, err := PromptSelectMatch(dir.Name(), input)
		if err != nil {
			// TODO add log message or customize error
			continue
		}

		// build input
		inputTags := []MusicTags{
			MusicTags{Tag: "Continue"},
			MusicTags{Tag: "Artist", Value: input[index].Artist},
			MusicTags{Tag: "Song name", Value: input[index].SongName},
			MusicTags{Tag: "Album", Value: input[index].Album},
		}

		// repeast until the user has had the opportunity to edit all tags
		// only "continue" will exit the loop
		for {
			// ID3 tags
			tag, err := id3v2.Open(pathToDir+"/"+dir.Name(), id3v2.Options{Parse: false})
			if err != nil {
				fmt.Printf("failed to parse mp3 id3 tags: %s", err.Error())
				continue
			}
			defer tag.Close()

			index, err = PromptSelectTag(dir.Name(), inputTags)
			if err != nil {
				break
			}

			if index == 0 {
				tag.SetArtist(inputTags[1].Value)
				tag.SetTitle(inputTags[2].Value)
				tag.SetAlbum(inputTags[3].Value)
				// persist new tags
				if err = tag.Save(); err != nil {
					fmt.Printf("failed to store tags to file: %s\n", err.Error())
				}
				break
			}

			newVal, err := PromptNewValue(inputTags[index].Value)
			if err != nil {
				break
			}

			inputTags[index].Value = newVal
		}
		if err != nil {
			fmt.Printf("failed handling user input: %s\n", err.Error())
			continue
		}
	}
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

func ParseFlags() string {
	directory := ""

	// using no default value to distinguish when nothing was passed
	flag.StringVar(&directory, "dir", "", "Music directory")

	flag.Parse()

	return directory
}