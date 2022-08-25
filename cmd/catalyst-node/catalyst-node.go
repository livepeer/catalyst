package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	serfclient "github.com/hashicorp/serf/client"
	"github.com/hashicorp/serf/cmd/serf/command/agent"
	"github.com/livepeer/livepeer-data/pkg/mistconnector"
	glog "github.com/magicsong/color-glog"
	"github.com/mitchellh/cli"
	"github.com/mitchellh/mapstructure"
	"github.com/peterbourgon/ff/v3"
)

const (
	httpPort         = 8090
	mistUtilLoadPort = 8042
)

var Version = "unknown"

type catalystConfig struct {
	serfRPCAddress           string
	serfRPCAuthKey           string
	serfTags                 map[string]string
	mistLoadBalancerEndpoint string
}

type catalystNodeCliFlags struct {
	Verbosity        int
	MistJSON         bool
	Version          bool
	RunBalancer      bool
	BalancerArgs     string
	HTTPAddress      string
	RedirectPrefixes []string
}

func runClient(config catalystConfig) error {
	client, err := connectSerfAgent(config.serfRPCAddress, config.serfRPCAuthKey)

	if err != nil {
		return err
	}
	defer client.Close()

	eventCh := make(chan map[string]interface{})
	streamHandle, err := client.Stream("*", eventCh)
	if err != nil {
		return fmt.Errorf("error starting stream: %w", err)
	}
	defer client.Stop(streamHandle)

	event := <-eventCh
	inbox := make(chan map[string]interface{}, 1)
	go func() {
		for {
			e := <-eventCh
			select {
			case inbox <- e:
				// Event is now in the inbox
			default:
				// Overflow event gets dropped
			}
		}
	}()

	// Ping the inbox initially and then every few seconds to retry on the load balancer
	go func() {
		for {
			e := map[string]interface{}{}
			select {
			case inbox <- e:
			default:
			}
			time.Sleep(5 * time.Second)
		}
	}()

	for {
		<-inbox
		glog.V(5).Infof("got event: %v", event)

		members, err := client.MembersFiltered(config.serfTags, ".*", ".*")

		if err != nil {
			glog.Errorf("Error getting serf, will retry: %v\n", err)
			continue
		}

		balancedServers, err := getMistLoadBalancerServers(config.mistLoadBalancerEndpoint)

		if err != nil {
			glog.Errorf("Error getting mist load balancer servers, will retry: %v\n", err)
			continue
		}

		membersMap := make(map[string]bool)

		for _, member := range members {
			memberHost := member.Name

			// commented out as for now the load balancer does not return ports
			//if member.Port != 0 {
			//	memberHost = fmt.Sprintf("%s:%d", memberHost, member.Port)
			//}

			membersMap[memberHost] = true
		}

		glog.V(5).Infof("current members in cluster: %v\n", membersMap)
		glog.V(5).Infof("current members in load balancer: %v\n", balancedServers)

		// compare membersMap and balancedServers
		// del all servers not present in membersMap but present in balancedServers
		// add all servers not present in balancedServers but present in membersMap

		// note: untested as per MistUtilLoad ports
		for k := range balancedServers {
			if _, ok := membersMap[k]; !ok {
				glog.Infof("deleting server %s from load balancer\n", k)
				_, err := changeLoadBalancerServers(config.mistLoadBalancerEndpoint, k, "del")
				if err != nil {
					glog.Errorf("Error deleting server %s from load balancer: %v\n", k, err)
				}
			}
		}

		for k := range membersMap {
			if _, ok := balancedServers[k]; !ok {
				glog.Infof("adding server %s to load balancer\n", k)
				_, err := changeLoadBalancerServers(config.mistLoadBalancerEndpoint, k, "add")
				if err != nil {
					glog.Errorf("Error adding server %s to load balancer: %v\n", k, err)
				}
			}
		}
	}
}

func connectSerfAgent(serfRPCAddress, serfRPCAuthKey string) (*serfclient.RPCClient, error) {
	return serfclient.ClientFromConfig(&serfclient.Config{
		Addr:    serfRPCAddress,
		AuthKey: serfRPCAuthKey,
	})
}

func changeLoadBalancerServers(endpoint, server, action string) ([]byte, error) {
	actionURL := endpoint + "?" + action + "server=" + url.QueryEscape(server)
	req, err := http.NewRequest("POST", actionURL, nil)
	if err != nil {
		glog.Errorf("Error creating request: %v", err)
		return nil, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		glog.Errorf("Error making request: %v", err)
		return nil, err
	}

	b, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		glog.Errorf("Error reading response: %v", err)
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		glog.Errorf("Error response from load balancer changing servers: %s\n", string(b))
		return b, errors.New(string(b))
	}

	glog.V(6).Infof("requested mist to %s server %s to the load balancer\n", action, server)
	glog.V(6).Info(string(b))
	return b, nil
}

func getMistLoadBalancerServers(endpoint string) (map[string]interface{}, error) {
	url := endpoint + "?lstservers=1"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		glog.Errorf("Error creating request: %v", err)
		return nil, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		glog.Errorf("Error making request: %v", err)
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		b, _ := ioutil.ReadAll(resp.Body)
		glog.Errorf("Error response from load balancer listing servers: %s\n", string(b))
		return nil, errors.New(string(b))
	}
	b, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		glog.Errorf("Error reading response: %v", err)
		return nil, err
	}

	var mistResponse map[string]interface{}

	json.Unmarshal([]byte(string(b)), &mistResponse)

	return mistResponse, nil
}

func execBalancer(balancerArgs []string) error {
	glog.Infof("Running MistUtilLoad with %v", balancerArgs)
	cmd := exec.Command("MistUtilLoad", balancerArgs...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		return err
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}

	return fmt.Errorf("MistUtilLoad exited cleanly")
}

func main() {
	var cliFlags = &catalystNodeCliFlags{}
	var config = &catalystConfig{}

	flag.Set("logtostderr", "true")
	vFlag := flag.Lookup("v")
	fs := flag.NewFlagSet("catalyst-node-connected", flag.ExitOnError)

	fs.BoolVar(&cliFlags.MistJSON, "j", false, "Print application info as json")
	fs.IntVar(&cliFlags.Verbosity, "v", 3, "Log verbosity.  {4|5|6}")
	fs.BoolVar(&cliFlags.Version, "version", false, "Print out the version")
	fs.BoolVar(&cliFlags.RunBalancer, "run-balancer", true, "run MistUtilLoad")
	fs.StringVar(&cliFlags.BalancerArgs, "balancer-args", "", "arguments passed to MistUtilLoad")
	prefixes := fs.String("redirect-prefixes", "", "Set of valid prefixes of playback id which are handled by mistserver")

	// Catalyst web server
	fs.StringVar(&cliFlags.HTTPAddress, "http-addr", fmt.Sprintf("127.0.0.1:%d", httpPort), "Address to bind for Catalyst HTTP commands")

	fs.StringVar(&config.serfRPCAddress, "serf-rpc-address", "127.0.0.1:7373", "Serf RPC address")
	fs.StringVar(&config.serfRPCAuthKey, "serf-rpc-auth-key", "", "Serf RPC auth key")
	serfTags := fs.String("serf-tags", "node=media", "Serf tags for Catalyst nodes")
	fs.StringVar(&config.mistLoadBalancerEndpoint, "mist-load-balancer-endpoint", "http://127.0.0.1:8042/", "Mist util load endpoint")

	// Serf commands passed straight through to the agent
	serfConfig := agent.Config{}
	fs.StringVar(&serfConfig.BindAddr, "bind", "0.0.0.0:9935", "Address to bind network listeners to. To use an IPv6 address, specify [::1] or [::1]:7946.")
	fs.StringVar(&serfConfig.RPCAddr, "rpc-addr", "127.0.0.1:7373", "Address to bind the RPC listener.")
	retryJoin := fs.String("retry-join", "", "An agent to join with. This flag be specified multiple times. Does not exit on failure like -join, used to retry until success.")
	fs.StringVar(&serfConfig.EncryptKey, "encrypt", "", "Key for encrypting network traffic within Serf. Must be a base64-encoded 32-byte key.")
	fs.StringVar(&serfConfig.Profile, "profile", "", "Profile is used to control the timing profiles used in Serf. The default if not provided is wan.")
	fs.StringVar(&serfConfig.NodeName, "node", "", "Name of this node. Must be unique in the cluster")

	ff.Parse(
		fs, os.Args[1:],
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
		ff.WithEnvVarPrefix("CATALYST_NODE"),
		ff.WithEnvVarSplit(","),
	)
	vFlag.Value.Set(fmt.Sprint(cliFlags.Verbosity))
	flag.CommandLine.Parse(nil)

	if cliFlags.MistJSON {
		mistconnector.PrintMistConfigJson(
			"catalyst-node",
			"Catalyst multi-node server. Coordinates stream replication and load balancing to multiple catalyst nodes.",
			"Catalyst Node",
			Version,
			fs,
		)
		return
	}

	if cliFlags.Version {
		fmt.Println("catalyst-node version: " + Version)
		fmt.Printf("golang runtime version: %s %s\n", runtime.Compiler, runtime.Version())
		fmt.Printf("architecture: %s\n", runtime.GOARCH)
		fmt.Printf("operating system: %s\n", runtime.GOOS)
		return
	}

	cliFlags.RedirectPrefixes = strings.Split(*prefixes, ",")
	glog.Infof("found redirectPrefixes=%v", cliFlags.RedirectPrefixes)

	parseSerfConfig(&serfConfig, retryJoin, serfTags)

	go startCatalystWebServer(cliFlags.HTTPAddress, cliFlags.RedirectPrefixes)

	config.serfTags = serfConfig.Tags

	if cliFlags.RunBalancer {
		go func() {
			err := execBalancer(strings.Split(cliFlags.BalancerArgs, " "))
			if err != nil {
				glog.Fatal(err)
			}
		}()
	}

	go func() {
		// eli note: i put this in a loop in case client boots before server.
		// doesn't seem to happen in practice.
		for {
			err := runClient(*config)
			if err != nil {
				glog.Errorf("Error starting client: %v", err)
			}
			time.Sleep(1 * time.Second)
		}
	}()

	// Everything past this is booting up Serf
	tmpFile, err := writeSerfConfig(&serfConfig)
	if err != nil {
		glog.Fatalf("Error writing serf config: %s", err)
	}
	defer os.Remove(tmpFile)
	glog.V(6).Infof("Wrote serf config to %s", tmpFile)

	ui := &cli.BasicUi{Writer: os.Stdout}

	// copied from:
	// https://github.com/hashicorp/serf/blob/a2bba5676d6e37953715ea10e583843793a0c507/cmd/serf/commands.go#L20-L25
	// we should consider invoking serf directly instead of wrapping their CLI helper
	commands := map[string]cli.CommandFactory{
		"agent": func() (cli.Command, error) {
			a := &agent.Command{
				Ui:         ui,
				ShutdownCh: make(chan struct{}),
			}
			return a, nil
		},
	}

	cli := &cli.CLI{
		Args:     []string{"agent", "-config-file", tmpFile},
		Commands: commands,
		HelpFunc: cli.BasicHelpFunc("catalyst-node"),
	}

	exitCode, err := cli.Run()
	if err != nil {
		glog.Fatalf("Error executing CLI: %s\n", err.Error())
		os.Exit(1)
	}

	os.Exit(exitCode)
}

func parseSerfConfig(config *agent.Config, retryJoin, serfTags *string) {
	if *retryJoin != "" {
		config.RetryJoin = strings.Split(*retryJoin, ",")
	}
	if *serfTags != "" {
		if config.Tags == nil {
			config.Tags = make(map[string]string)
		}
		for _, t := range strings.Split(*serfTags, ",") {
			kv := strings.Split(t, "=")
			if len(kv) == 2 {
				k, v := kv[0], kv[1]
				config.Tags[k] = v
			} else {
				glog.Fatalf("failed to parse serf tag, --serf-tag=k1=v1,k2=v2 format required: %s", t)
			}
		}
	}
}

func writeSerfConfig(config *agent.Config) (string, error) {
	// Two steps to properly serialize this as JSON: https://github.com/spf13/viper/issues/816#issuecomment-1149732004
	items := map[string]interface{}{}
	if err := mapstructure.Decode(config, &items); err != nil {
		return "", err
	}
	b, err := json.Marshal(items)

	if err != nil {
		return "", err
	}

	// Everything after this is booting up serf with our provided config flags:
	tmpFile, err := ioutil.TempFile(os.TempDir(), "serf-config-*.json")
	if err != nil {
		return "", err
	}

	// Example writing to the file
	if _, err = tmpFile.Write(b); err != nil {
		return "", err
	}

	// Close the file
	if err := tmpFile.Close(); err != nil {
		return "", err
	}

	return tmpFile.Name(), err
}

func startCatalystWebServer(httpAddr string, redirectPrefixes []string) {
	http.Handle("/hls/", redirectHlsHandler(redirectPrefixes))
	glog.Infof("HTTP server listening on %s", httpAddr)
	glog.Fatal(http.ListenAndServe(httpAddr, nil))
}

var getClosestNode = queryMistForClosestNode

func redirectHlsHandler(redirectPrefixes []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		playbackID, isValid := parsePlaybackID(r.URL.Path)
		if !isValid {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		lat := r.Header.Get("X-Latitude")
		lon := r.Header.Get("X-Longitude")

		var nodeAddr, validPrefix string
		var err error

		for _, prefix := range redirectPrefixes {
			nodeAddr, err = getClosestNode(playbackID, lat, lon, prefix)
			if err != nil {
				glog.Errorf("error finding origin server playbackID=%s prefix=%s error=%s", playbackID, prefix, err)
				continue
			}
			if nodeAddr != "" {
				validPrefix = prefix + "+"
				err = nil
				break
			}
		}

		if nodeAddr == "" || err != nil {
			glog.Errorf("error finding origin server playbackID=%s error=%s", playbackID, err)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		rURL := fmt.Sprintf("%s://%s/hls/%s%s/index.m3u8", protocol(r), nodeAddr, validPrefix, playbackID)
		http.Redirect(w, r, rURL, http.StatusFound)
	})
}

func parsePlaybackID(path string) (string, bool) {
	r := regexp.MustCompile("^/hls/([\\w+-]+)/index.m3u8$")
	m := r.FindStringSubmatch(path)
	if len(m) < 2 {
		return "", false
	}
	// Incoming requests might come with some prefix attached to the
	// playback ID. We try to drop that here by splitting at `+` and
	// picking the last piece. For eg.
	// incoming path = '/hls/video+4712oox4msvs9qsf/index.m3u8'
	// playbackID = '4712oox4msvs9qsf'
	slice := strings.Split(m[1], "+")
	return slice[len(slice)-1], true
}

func protocol(r *http.Request) string {
	if r.Header.Get("X-Forwarded-Proto") == "https" {
		return "https"
	}
	return "http"
}

func queryMistForClosestNode(playbackID, lat, lon, prefix string) (string, error) {
	url := fmt.Sprintf("http://localhost:%d/%s+%s", mistUtilLoadPort, prefix, playbackID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	if lat != "" && lon != "" {
		req.Header.Set("X-Latitude", lat)
		req.Header.Set("X-Longitude", lon)
	} else {
		glog.Warningf("Incoming request missing X-Latitude/X-Longitude, response will not be geolocated")
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET request '%s' failed with http status code %d", url, resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("GET request '%s' failed while reading response body", url)
	}
	if string(body) == "FULL" {
		return "", fmt.Errorf("GET request '%s' returned 'FULL'", url)
	}
	return string(body), nil
}
