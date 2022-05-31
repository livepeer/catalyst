package main

import (
	"fmt"
	"os"
	"time"

	serfclient "github.com/hashicorp/serf/client"
	"github.com/hashicorp/serf/cmd/serf/command/agent"
	"github.com/mitchellh/cli"
)

var Commands map[string]cli.CommandFactory

func init() {
	ui := &cli.BasicUi{Writer: os.Stdout}

	// eli note: this is copied from here:
	// https://github.com/hashicorp/serf/blob/a2bba5676d6e37953715ea10e583843793a0c507/cmd/serf/commands.go#L20-L25
	// but we should someday get a little bit smarter and invoke serf directly
	// instead of wrapping their CLI helper

	Commands = map[string]cli.CommandFactory{
		"agent": func() (cli.Command, error) {
			a := &agent.Command{
				Ui:         ui,
				ShutdownCh: make(chan struct{}),
			}
			return a, nil
		},
	}
}

func runClient() error {
	// eli note: hardcoded. needs to parse out configuration from the CLI
	config := serfclient.Config{Addr: "127.0.0.1:7373", AuthKey: ""}
	client, err := serfclient.ClientFromConfig(&config)

	if err != nil {
		return err
	}
	defer client.Close()

	eventCh := make(chan map[string]interface{}, 1024)
	streamHandle, err := client.Stream("*", eventCh)
	if err != nil {
		return fmt.Errorf("error starting stream: %s", err)
	}
	defer client.Stop(streamHandle)

	// eli note: not sure if this is useful yet, but we can do this as well:

	// logCh := make(chan string, 1024)
	// monHandle, err := client.Monitor(logutils.LogLevel("INFO"), logCh)
	// if err != nil {
	// 	return fmt.Errorf("error starting monitor: %s", err)
	// }
	// defer client.Stop(monHandle)

	// eli note: uncertain how we handle dis/reconnects here. but it's local, so hopefully rare?
	for {
		event := <-eventCh
		fmt.Printf("got event: %v\n", event)
		// eli note: this is the part where we trigger reconcillation with Mist
	}

	return nil
}

func main() {
	args := os.Args[1:]

	go func() {
		// eli note: i put this in a loop in case client boots before server.
		// doesn't seem to happen in practice.
		for {
			err := runClient()
			if err != nil {
				fmt.Printf("Error starting client: %v", err)
			}
			time.Sleep(1 * time.Second)
		}
	}()

	cli := &cli.CLI{
		Args:     args,
		Commands: Commands,
		HelpFunc: cli.BasicHelpFunc("catalyst-node"),
	}

	exitCode, err := cli.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing CLI: %s\n", err.Error())
		os.Exit(1)
	}

	os.Exit(exitCode)
}
