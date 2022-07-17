package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/manifoldco/promptui"
)

func AskForDirectory() (string, error) {
	currentDir, err := os.ReadDir(".")
	if err != nil {
		fmt.Printf("failed to read current directly: %s", err.Error())
		return "", err
	}

	subDirs := []string{"."}

	for _, dir := range currentDir {
		if !dir.IsDir() {
			continue
		}
		subDirs = append(subDirs, dir.Name())
	}

	if len(subDirs) == 1 {
		return ".", nil
	}

	prompt := promptui.Select{
		Label: "Select directory",
		Items: subDirs,
	}

	_, result, err := prompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return "", err
	}
	return result, nil
}

func GetFpcalcPath() string {
	fpcalc := os.Getenv("FPCALC_BINARY_PATH")
	if fpcalc == "" {
		var err error
		fpcalc, err = exec.LookPath("fpcalc")
		if err != nil {
			// TODO check error is due to not finding the binary
			// fallback to the current dir
			fpcalc = "./fpcalc"
		}
	}
	return fpcalc
}
