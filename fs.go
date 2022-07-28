package main

import (
	"os"
	"os/exec"
)

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
