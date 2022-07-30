package main

import (
	"errors"
	"os"
	"os/exec"
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
