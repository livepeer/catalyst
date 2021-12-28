package main

import (
	"fmt"
	"os"

	"github.com/livepeer/go-livepeer/cmd/livepeer"
	"github.com/livepeer/livepeer-data/cmd/analyzer"
)

func main() {
	switch os.Args[1] {
	case "analyzer":
		os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
		analyzer.Run(analyzer.BuildFlags{
			Version: "box",
		})
	case "go-livepeer":
		os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
		livepeer.Run()
	default:
		fmt.Printf("Unknown subcommand: '%s'\n", os.Args[1])
		os.Exit(1)
	}
}
