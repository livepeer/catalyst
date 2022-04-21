package main

import (
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
)

//go:embed conf
var conf embed.FS

type CLI struct {
	Mode, APIKey, EthOrchAddr, DataDir string
}

func main() {
	flag.Set("logtostderr", "true")
	vFlag := flag.Lookup("v")
	fs := flag.NewFlagSet("catalyst", flag.ExitOnError)

	cli := CLI{}

	verbosity := fs.String("v", "", "Log verbosity.  {4|5|6}")
	fs.StringVar(&cli.Mode, "mode", "", "Allowed options: local, api, mainnet")
	fs.StringVar(&cli.APIKey, "apiKey", "", "With --mode=api, which Livepeer.com API key should you use?")
	fs.StringVar(&cli.EthOrchAddr, "ethOrchAddr", "", "With --mode=mainnet, the Ethereum address of a hardcoded orchestrator")
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

	glog.V(6).Infof("mode=%s apiKey=%s ethOrchAddr=%s", cli.Mode, cli.APIKey, cli.EthOrchAddr)

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
	if _, err := os.Stat(cli.DataDir + "/not-mounted"); !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("settings directory not bind-mounted. Please run with -v ./livepeer:%s", cli.DataDir)
	}

	confPath := fmt.Sprintf("%s/catalyst.json", cli.DataDir)

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
