package main

import (
	"flag"
	"os"

	"github.com/golang/glog"
	ff "github.com/peterbourgon/ff/v3"
)

func main() {
	flag.Set("logtostderr", "true")
	vFlag := flag.Lookup("v")
	fs := flag.NewFlagSet("catalyst", flag.ExitOnError)

	verbosity := fs.String("v", "", "Log verbosity.  {4|5|6}")
	mode := fs.String("mode", "", "Allowed options: local, api, mainnet")
	apiKey := fs.String("apiKey", "", "With --mode=api, which Livepeer.com API key should you use?")
	ethOrchAddr := fs.String("ethOrchAddr", "", "With --mode=mainnet, the Ethereum address of a hardcoded orchestrator")

	ff.Parse(
		fs, os.Args[1:],
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
		ff.WithEnvVarPrefix("CATALYST"),
		ff.WithEnvVarSplit(","),
	)
	flag.CommandLine.Parse(nil)
	vFlag.Value.Set(*verbosity)

	glog.Infof("%s %s %s", *mode, *apiKey, *ethOrchAddr)
}
