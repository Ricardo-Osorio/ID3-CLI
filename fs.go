package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/bogem/id3v2"
)

// GetFpcalcPath returns the path to fpcalc and does so in
// the following order:
//  - Use value of the env variable "FPCALC_BINARY_PATH"
//  - Search in the PATH (equivalent to `which fpcalc`)
//  - Fallback to the project root "./fpcalc"
func GetFpcalcPath() (string, error) {
	fpcalc := os.Getenv("FPCALC_BINARY_PATH")
	if fpcalc != "" {
		return fpcalc, nil
	}

	fpcalc, err := exec.LookPath("fpcalc")
	if err == nil {
		return fpcalc, nil
	}

	if errors.Is(err, exec.ErrNotFound) {
		return "./fpcalc", nil
	}
	return "", err
}

// CommitChangesToFile is the final step to updating a file. It updates
// the id3 tags and renames it according to its matched result.
func CommitChangesToFile(file *id3v2.Tag, artist, songName, album, fileName string) error {
	file.SetArtist(artist)
	file.SetTitle(songName)
	file.SetAlbum(album)
	// persist new tags
	if err := file.Save(); err != nil {
		fmt.Printf("failed to store tags: %s\n", err.Error())
		return err
	}

	if !renameFiles {
		return nil
	}

	os.Rename(pathToMusic+"/"+fileName, fmt.Sprintf("%s/%s - %s.mp3", pathToMusic, artist, songName))
	fmt.Printf("Renamed file from \"%s\" to \"%s - %s.mp3\"\n", fileName, artist, songName)
	return nil
}
