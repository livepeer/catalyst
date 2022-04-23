package main

import (
	"bufio"
	"embed"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"os"
	"syscall"

	"github.com/golang/glog"
	"github.com/livepeer/livepeer-in-a-box/internal/utils"
	ff "github.com/peterbourgon/ff/v3"
	"golang.org/x/term"
)

//go:embed conf
var conf embed.FS

type CLI struct {
	Mode, APIKey, OrchAddr, DataDir, EthPassword, EthURL string
}

func main() {
	flag.Set("logtostderr", "true")
	vFlag := flag.Lookup("v")
	fs := flag.NewFlagSet("catalyst", flag.ExitOnError)

	cli := CLI{}

	verbosity := fs.String("v", "", "Log verbosity.  {4|5|6}")
	fs.StringVar(&cli.Mode, "mode", "", "Allowed options: local, api, mainnet")
	fs.StringVar(&cli.APIKey, "apiKey", "", "With --mode=api, which Livepeer.com API key should you use?")
	fs.StringVar(&cli.OrchAddr, "orchAddr", "", "With --mode=mainnet, the Ethereum address of a hardcoded orchestrator")
	fs.StringVar(&cli.EthURL, "ethUrl", "", "Address of an Arbitrum Mainnet HTTP-RPC node")
	fs.StringVar(&cli.EthPassword, "ethPassword", "", "With --mode=mainnet, password for mounted Ethereum wallet. Will be prompted if not provided.")
	fs.StringVar(&cli.DataDir, "dataDir", "/etc/livepeer", "Directory within the container to save settings")

	ff.Parse(
		fs, os.Args[1:],
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
		ff.WithEnvVarPrefix("CATALYST"),
		ff.WithEnvVarSplit(","),
	)
	flag.CommandLine.Parse(nil)
	vFlag.Value.Set(*verbosity)

	confPath, err := ensureConfigFile(cli)
	if err != nil {
		glog.Fatalf("error creating config file: %s", err)
	}

	execErr := syscall.Exec("/usr/bin/MistController", []string{"MistController", "-c", confPath}, []string{})
	if execErr != nil {
		glog.Fatal(execErr)
	}
}

func ensureConfigFile(cli CLI) (string, error) {
	if utils.IsFileExists(cli.DataDir + "/not-mounted") {
		return "", fmt.Errorf("settings directory not bind-mounted. Please run with -v ./livepeer:%s", cli.DataDir)
	}

	confPath := fmt.Sprintf("%s/catalyst.json", cli.DataDir)

	if cli.Mode == "mainnet" && cli.EthURL == "" {
		return "", fmt.Errorf("--ethUrl is required with --mode=mainnet")
	}

	if cli.Mode == "mainnet" && cli.EthPassword == "" {
		keystoreExists := utils.IsFileExists(cli.DataDir + "/mainnet-broadcaster/keystore")
		if !keystoreExists {
			glog.Infof("No Ethereum account detected, creating new account.")
		}
		fmt.Print("Enter password for Ethereum account: ")
		passphraseBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println("")
		passphrase := string(passphraseBytes)
		if err != nil {
			return "", err
		}
		if !keystoreExists {
			fmt.Print("Confirm password for new Ethereum account: ")
			confirmPassphraseBytes, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println("")
			confirmPassphrase := string(confirmPassphraseBytes)
			if err != nil {
				return "", err
			}
			if passphrase != confirmPassphrase {
				return "", fmt.Errorf("passphrase did not match")
			}
		}
		pwFile, err := os.Create("/tmp/pw.conf")
		if err != nil {
			return "", err
		}
		defer pwFile.Close()
		w := bufio.NewWriter(pwFile)
		_, err = w.WriteString("ethPassword " + passphrase)
		if err != nil {
			return "", err
		}

		w.Flush()
	}

	if utils.IsFileExists(confPath) {
		// Already exists
		glog.Infof("found %s, skipping config generation", confPath)
		return confPath, nil
	}

	glog.Infof("First boot detected, generating config file...")
	blob, err := conf.ReadFile(fmt.Sprintf("conf/catalyst.%s.json", cli.Mode))
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("--mode %s not found. Allowed values: local, api, mainnet", cli.Mode)
	} else if err != nil {
		return "", fmt.Errorf("error loading catalyst.%s.json: %s", cli.Mode, err)
	}

	// mode-specific checks
	if cli.Mode == "api" && cli.APIKey == "" {
		return "", fmt.Errorf("--apiKey is required in --mode=api")
	}

	tmpl := string(blob)
	f, err := os.Create(confPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	t, err := template.New("conf").Parse(tmpl)
	if err != nil {
		return "", err
	}
	err = t.Execute(f, cli)
	if err != nil {
		return "", err
	}
	glog.Infof("wrote %s", confPath)

	return confPath, nil
}
