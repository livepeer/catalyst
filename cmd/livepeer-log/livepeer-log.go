package main

// Helper binary to wrap binaries that don't implement Mist's
// logging format and output something that Mist can print
// nicely.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"

	"github.com/livepeer/livepeer-data/pkg/mistconnector"
)

func main() {
	// Pass through all command line arguments except the first one
	procname := path.Base(os.Args[1])
	// panic(fmt.Sprintf("===== %s\n", strings.Join(os.Args, " ")))
	rest := os.Args[2:]
	// If we're being called for our OWN -j, respond accordingly
	if len(rest) == 0 && procname == "-j" {
		printJSONInfo(mistconnector.MistConfig{
			Name:         "Livepeer Logger",
			FriendlyName: "Logger for livepeer-* applications",
			Description:  "Logger for other livepeer-whatever applications. No need to add directly.",
			Version:      "0.0.1",
		})
		os.Exit(0)
	}
	dashJ := false
	for _, seg := range rest {
		if seg == "-j" {
			dashJ = true
		}
	}
	if procname == "livepeer-victoria-metrics" && dashJ {
		printJSONInfo(mistconnector.MistConfig{
			Name:         "Livepeer Victoria Metrics",
			FriendlyName: "Livepeer-in-a-Box packaged Victoria Metrics",
			Description:  "Livepeer-in-a-Box packaged Victoria Metrics. Comes with some built-in scrape configs for dev.",
			Version:      "0.0.1",
			Required: map[string]mistconnector.MistOptional{
				"promscrape.config": {
					Name:    "promscrape.config",
					Type:    "str",
					Option:  "-promscrape.config",
					Help:    "Location of promscape.config file",
					Default: "./config/scrape_config.yaml",
				},
			},
		})
		os.Exit(0)
	}
	mypid := os.Getpid()
	cmd := exec.Command(os.Args[1], rest...)
	cmd.Stdin = os.Stdin

	if dashJ {
		// If they did -j, don't mangle the output
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		// Set up line-by-line parsing of stdout and stderr
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			panic(err)
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			panic(err)
		}

		// Actually print with lots of lines!
		for _, pipe := range []io.ReadCloser{stderr, stdout} {
			go func(pipe io.ReadCloser) {
				scanner := bufio.NewScanner(pipe)
				for scanner.Scan() {
					text := scanner.Text()
					fmt.Fprintf(os.Stderr, "INFO|%s|%d|||%s\n", procname, mypid, text)
				}
			}(pipe)
		}

	}

	// Forward all signals to the subprocess
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan)

		// You need a for loop to handle multiple signals
		for {
			sig := <-sigChan
			if err := cmd.Process.Signal(sig); err != nil {
				// Appears to be a mac-only issue where this fires before cmd.Run() returns
				if err.Error() == "os: process already finished" {
					break
				}
				panic(err)
			}
		}
	}()

	// Fire away!
	progerr := cmd.Run()
	if progerr != nil {
		newerr := fmt.Sprintf("%s - invocation %s", progerr.Error(), strings.Join(os.Args, " "))
		panic(newerr)
	}
	os.Exit(0)
}

func printJSONInfo(jsonInfo mistconnector.MistConfig) {
	blob, err := json.Marshal(jsonInfo)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(blob))
}
