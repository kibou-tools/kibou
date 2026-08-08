package main

import (
	"encoding/json"
	"os"
	"runtime"
)

type versionOutput struct {
	Runtime string `json:"runtime"`
}

func main() {
	output := versionOutput{Runtime: runtime.Version()}
	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		panic(err)
	}
}
