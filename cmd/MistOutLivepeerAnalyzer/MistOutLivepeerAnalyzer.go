package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/livepeer/livepeer-data/cmd/analyzer"
)

type mistOptions struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-j" {
		str, err := json.Marshal(mistOptions{
			Name:    "LivepeerAnalyzer",
			Version: "2.18.1-Pro-313-gbe6a170d2",
		})
		if err != nil {
			panic(err)
		}
		fmt.Println(string(str))
	} else {
		fmt.Println("Booting up!")
		analyzer.Run(analyzer.BuildFlags{
			Version: "box",
		})
	}
}
