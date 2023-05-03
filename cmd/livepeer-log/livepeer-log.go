package main

// Helper binary to wrap binaries that don't implement Mist's
// logging format and output something that Mist can print
// nicely.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"

	"github.com/livepeer/livepeer-data/pkg/mistconnector"
	"golang.org/x/sync/errgroup"
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
			Optional: map[string]mistconnector.MistOptional{
				"promscrape.config": {
					Name:    "promscrape.config",
					Type:    "str",
					Option:  "-promscrape.config",
					Help:    "Location of promscape.config file",
					Default: "./config/scrape_config.yaml",
				},
				"envflag.enable": {
					Name:    "envflag.enable",
					Option:  "-envflag.enable",
					Help:    "Whether to enable reading flags from environment variables additionally to command line. Command line flag values have priority over values from environment vars. Flags are read only from command line if this flag isn't set. See https://docs.victoriametrics.com/#environment-variables for more details",
					Default: "true",
				},
				"envflag.prefix": {
					Name:    "envflag.prefix",
					Type:    "str",
					Option:  "-envflag.prefix",
					Help:    "Prefix for environment variables if -envflag.enable is set",
					Default: "VM_",
				},
			},
		})
		os.Exit(0)
	}

	if procname == "livepeer-vmagent" && dashJ {
		printJSONInfo(mistconnector.MistConfig{
			Name:         "Livepeer Victoria Metrics Agent",
			FriendlyName: "Livepeer-in-a-Box packaged Victoria Metrics Agent (exporter)",
			Description:  "Livepeer-in-a-Box packaged Victoria Metrics. Useful for remote writing metrics.",
			Version:      "0.0.1",
			Optional: map[string]mistconnector.MistOptional{
				"promscrape.config": {
					Name:    "promscrape.config",
					Type:    "str",
					Option:  "-promscrape.config",
					Help:    "Location of promscape.config file",
					Default: "./config/scrape_config.yaml",
				},
				"remoteWrite.url": {
					Name:    "remoteWrite.url",
					Type:    "str",
					Option:  "-remoteWrite.url",
					Help:    "array of urls of the Victoria Metrics remote endpoint",
					Default: "http://localhost/",
				},
				"remoteWrite.label": {
					Name:    "remoteWrite.label",
					Type:    "str",
					Option:  "-remoteWrite.label",
					Help:    "array of labels to add to metrics example label=value,label2=value2",
					Default: "region=dev",
				},
				"loggerLevel": {
					Name:    "loggerLevel",
					Type:    "str",
					Option:  "-loggerLevel",
					Help:    "Minimum level of errors to log. Possible values: INFO, WARN, ERROR, FATAL, PANIC (default 'INFO')",
					Default: "FATAL",
				},
				"envflag.enable": {
					Name:    "envflag.enable",
					Option:  "-envflag.enable",
					Help:    "Whether to enable reading flags from environment variables additionally to command line. Command line flag values have priority over values from environment vars. Flags are read only from command line if this flag isn't set. See https://docs.victoriametrics.com/#environment-variables for more details",
					Default: "true",
				},
				"envflag.prefix": {
					Name:    "envflag.prefix",
					Type:    "str",
					Option:  "-envflag.prefix",
					Help:    "Prefix for environment variables if -envflag.enable is set",
					Default: "VM_",
				},
			},
		})
		os.Exit(0)
	}

	mypid := os.Getpid()
	cmd := exec.Command(os.Args[1], rest...)
	cmd.Stdin = os.Stdin

	group, _ := errgroup.WithContext(context.Background())

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

		output := func(text string) {
			fmt.Fprintf(os.Stderr, "INFO|%s|%d|||%s\n", procname, mypid, text)
		}

		// Actually print with lots of lines!
		for i, pipe := range []io.ReadCloser{stdout, stderr} {
			func(i int, pipe io.ReadCloser) {
				group.Go(func() error {
					reader := bufio.NewReader(pipe)

					for {
						line, isPrefix, err := reader.ReadLine()
						if string(line) != "" {
							output(string(line))
						}
						if err != nil {
							output(fmt.Sprintf("reader gave error, ending logging for fd=%d err=%s", i+1, err))
							line, _, err := reader.ReadLine()
							output(string(line))
							return err
						}
						if isPrefix {
							output("warning: preceeding line exceeds 64k logging limit and was split")
						}
					}
				})
			}(i, pipe)
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
	progerr := cmd.Start()
	if progerr != nil {
		newerr := fmt.Errorf("%s - invocation %s", progerr.Error(), strings.Join(os.Args, " "))
		panic(newerr)
	}

	group.Wait()

	progerr = cmd.Wait()
	if progerr != nil {
		newerr := fmt.Errorf("%s - invocation %s", progerr.Error(), strings.Join(os.Args, " "))
		panic(newerr)
	}
}

func printJSONInfo(jsonInfo mistconnector.MistConfig) {
	blob, err := json.Marshal(jsonInfo)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(blob))
}
