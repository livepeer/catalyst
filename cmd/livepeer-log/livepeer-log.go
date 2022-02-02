package main

// Helper binary to wrap binaries that don't implement Mist's
// logging format and output something that Mist can print
// nicely.

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"
)

func main() {
	// Pass through all command line arguments except the first one
	procname := path.Base(os.Args[1])
	// panic(fmt.Sprintf("===== %s\n", strings.Join(os.Args, " ")))
	rest := os.Args[2:]
	dashJ := false
	for _, seg := range rest {
		if seg == "-j" {
			dashJ = true
		}
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
}
