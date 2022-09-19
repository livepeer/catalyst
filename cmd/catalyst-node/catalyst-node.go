package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
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
	httpInternalPort = 8091
	mistUtilLoadPort = 8042
)

var Version = "unknown"
var serfClient *serfclient.RPCClient

type catalystConfig struct {
	serfRPCAddress           string
	serfRPCAuthKey           string
	serfTags                 map[string]string
	mistLoadBalancerEndpoint string
	mistLoadBalancerTemplate string
}

type catalystNodeCliFlags struct {
	Verbosity           int
	MistJSON            bool
	Version             bool
	RunBalancer         bool
	BalancerArgs        string
	HTTPAddress         string
	HTTPInternalAddress string
	RedirectPrefixes    []string
	NodeHost            string
}

var mediaFilter = map[string]string{"node": "media"}

func runClient(config catalystConfig) error {
	client, err := connectSerfAgent(config.serfRPCAddress, config.serfRPCAuthKey)

	if err != nil {
		return err
	}
	serfClient = client
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

		members, err := client.MembersFiltered(mediaFilter, ".*", ".*")

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
				_, err := changeLoadBalancerServers(config.mistLoadBalancerEndpoint, config.mistLoadBalancerTemplate, k, "del")
				if err != nil {
					glog.Errorf("Error deleting server %s from load balancer: %v\n", k, err)
				}
			}
		}

		for k := range membersMap {
			if _, ok := balancedServers[k]; !ok {
				glog.Infof("adding server %s to load balancer\n", k)
				_, err := changeLoadBalancerServers(config.mistLoadBalancerEndpoint, config.mistLoadBalancerTemplate, k, "add")
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

func changeLoadBalancerServers(endpoint, tmpl, server, action string) ([]byte, error) {
	serverTmpl := fmt.Sprintf(tmpl, server)
	actionURL := endpoint + "?" + action + "server=" + url.QueryEscape(serverTmpl)
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

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
		for {
			s := <-c
			glog.Errorf("caught signal=%v killing MistUtilLoad", s)
			cmd.Process.Kill()
		}
	}()

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
	fs.StringVar(&cliFlags.NodeHost, "node-host", "", "Hostname this node should handle requests for. Requests on any other domain will trigger a redirect. Useful as a 404 handler to send users to another node.")
	prefixes := fs.String("redirect-prefixes", "", "Set of valid prefixes of playback id which are handled by mistserver")

	// Catalyst web server
	fs.StringVar(&cliFlags.HTTPAddress, "http-addr", fmt.Sprintf("127.0.0.1:%d", httpPort), "Address to bind for external-facing Catalyst HTTP handling")
	fs.StringVar(&cliFlags.HTTPInternalAddress, "http-internal-addr", fmt.Sprintf("127.0.0.1:%d", httpInternalPort), "Address to bind for internal privileged HTTP commands")

	fs.StringVar(&config.serfRPCAddress, "serf-rpc-address", "127.0.0.1:7373", "Serf RPC address")
	fs.StringVar(&config.serfRPCAuthKey, "serf-rpc-auth-key", "", "Serf RPC auth key")
	serfTags := fs.String("serf-tags", "node=media", "Serf tags for Catalyst nodes")
	fs.StringVar(&config.mistLoadBalancerEndpoint, "mist-load-balancer-endpoint", "http://127.0.0.1:8042/", "Mist util load endpoint")
	fs.StringVar(&config.mistLoadBalancerTemplate, "mist-load-balancer-template", "http://%s:4242", "template for passing nodes to MistUtilBalancer")

	// Serf commands passed straight through to the agent
	serfConfig := agent.Config{}
	fs.StringVar(&serfConfig.BindAddr, "bind", "0.0.0.0:9935", "Address to bind network listeners to. To use an IPv6 address, specify [::1] or [::1]:7946.")
	fs.StringVar(&serfConfig.AdvertiseAddr, "advertise", "0.0.0.0", "Address to advertise to the other cluster members")
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
	glog.V(4).Infof("found redirectPrefixes=%v", cliFlags.RedirectPrefixes)

	parseSerfConfig(&serfConfig, retryJoin, serfTags)

	go startCatalystWebServer(cliFlags.HTTPAddress, cliFlags.RedirectPrefixes, cliFlags.NodeHost)
	go startInternalWebServer(cliFlags.HTTPInternalAddress)

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

func startCatalystWebServer(httpAddr string, redirectPrefixes []string, nodeHost string) {
	http.Handle("/", redirectHandler(redirectPrefixes, nodeHost))
	glog.Infof("HTTP server listening on %s", httpAddr)
	glog.Fatal(http.ListenAndServe(httpAddr, nil))
}

func startInternalWebServer(internalAddr string) {
	http.Handle("/STREAM_SOURCE", streamSourceHandler())
	glog.Infof("Internal HTTP server listening on %s", internalAddr)
	glog.Fatal(http.ListenAndServe(internalAddr, nil))
}

var getClosestNode = queryMistForClosestNode

func redirectHandler(redirectPrefixes []string, nodeHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")

		if nodeHost != "" {
			host := r.Host
			if host != nodeHost {
				rURL := fmt.Sprintf("%s://%s%s", protocol(r), nodeHost, r.URL.Path)
				http.Redirect(w, r, rURL, http.StatusFound)
				glog.V(6).Infof("NodeHost redirect host=%s nodeHost=%s from=%s to=%s", host, nodeHost, r.URL, rURL)
				return
			}
		}

		playbackID, pathTmpl, isValid := parsePlaybackID(r.URL.Path)
		if !isValid {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		lat := r.Header.Get("X-Latitude")
		lon := r.Header.Get("X-Longitude")

		bestNode, fullPlaybackID, err := getBestNode(redirectPrefixes, playbackID, lat, lon)
		if err != nil {
			glog.Errorf("failed to find either origin or fallback server for playbackID=%s err=%s", playbackID, err)
			w.WriteHeader(http.StatusBadGateway)
			return
		}

		rPath := fmt.Sprintf(pathTmpl, fullPlaybackID)
		rURL := fmt.Sprintf("%s://%s%s", protocol(r), bestNode, rPath)
		rURL, err = resolveNodeURL(rURL)
		if err != nil {
			glog.Errorf("failed to resolve node URL playbackID=%s err=%s", playbackID, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		glog.V(6).Infof("generated redirect url=%s", rURL)
		http.Redirect(w, r, rURL, http.StatusFound)
	})
}

func streamSourceHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			glog.Errorf("error handling STREAM_SOURCE body=%s", err)
			w.Write([]byte("push://"))
			return
		}
		streamName := string(b)
		glog.V(7).Infof("got mist STREAM_SOURCE request=%s", streamName)
		dtscURL, err := queryMistForClosestNodeSource(streamName, "0", "0", "", true)
		if err != nil {
			glog.Errorf("error querying mist for STREAM_SOURCE: %s", err)
			w.Write([]byte("push://"))
			return
		}
		outURL, err := resolveNodeURL(dtscURL)
		if err != nil {
			glog.Errorf("error finding STREAM_SOURCE: %s", err)
			w.Write([]byte("push://"))
			return
		}
		glog.V(7).Infof("replying to Mist STREAM_SOURCE request=%s response=%s", streamName, outURL)
		w.Write([]byte(outURL))
	})
}

// Given a dtsc:// or https:// url, resolve the proper address of the node via serf tags
func resolveNodeURL(streamURL string) (string, error) {
	u, err := url.Parse(streamURL)
	if err != nil {
		return "", err
	}
	nodeName := u.Host
	protocol := u.Scheme

	member, err := getSerfMember(nodeName)
	addr, has := member.Tags[protocol]
	if !has {
		glog.V(7).Infof("no tag found, not tag resolving protocol=%s nodeName=%s", protocol, nodeName)
		return streamURL, nil
	}
	u2, err := url.Parse(addr)
	if err != nil {
		glog.Errorf("node has unparsable tag!! nodeName=%s protocol=%s tag=%s", nodeName, protocol, addr)
	}
	u2.Path = u.Path
	u2.RawQuery = u.RawQuery
	return u2.String(), nil
}

func querySerfForMember(name string) (*serfclient.Member, error) {
	members, err := serfClient.MembersFiltered(map[string]string{}, "alive", name)
	if err != nil {
		return nil, err
	}
	if len(members) < 1 {
		return nil, fmt.Errorf("could not find serf member name=%s", name)
	}
	if len(members) > 1 {
		glog.Errorf("WARNING: found multiple serf members with the same name! name=%s count=%d", name, len(members))
	}
	return &members[0], nil
}

var getSerfMember = querySerfForMember

// return the best node available for a given stream. will return any node if nobody has the stream.
func getBestNode(redirectPrefixes []string, playbackID, lat, lon string) (string, string, error) {
	var nodeAddr, fullPlaybackID, fallbackAddr string
	var mu sync.Mutex
	var err error
	var waitGroup sync.WaitGroup

	for _, prefix := range redirectPrefixes {
		waitGroup.Add(1)
		go func(prefix string) {
			addr, e := getClosestNode(playbackID, lat, lon, prefix)
			mu.Lock()
			defer mu.Unlock()
			if e != nil {
				err = e
				glog.V(8).Infof("error finding origin server playbackID=%s prefix=%s error=%s", playbackID, prefix, e)
				// If we didn't find a stream but we did find a server, keep that so we can use it to handle a 404
				if addr != "" {
					fallbackAddr = addr
				}
			} else {
				nodeAddr = addr
				fullPlaybackID = prefix + "+" + playbackID
			}
			waitGroup.Done()
		}(prefix)
	}
	waitGroup.Wait()

	// good path: we found the stream and a good node to play it back, yay!
	if nodeAddr != "" {
		return nodeAddr, fullPlaybackID, nil
	}

	// bad path: nobody has the stream, but we did find a server which can handle the 404 for us.
	if fallbackAddr != "" {
		return fallbackAddr, redirectPrefixes[0] + "+" + playbackID, nil
	}

	// ugly path: we couldn't find ANY servers. yikes.
	return "", "", err
}

// Incoming requests might come with some prefix attached to the
// playback ID. We try to drop that here by splitting at `+` and
// picking the last piece. For eg.
// incoming path = '/hls/video+4712oox4msvs9qsf/index.m3u8'
// playbackID = '4712oox4msvs9qsf'
func parsePlaybackIDHLS(path string) (string, string, bool) {
	r := regexp.MustCompile(`^/hls/([\w+-]+)/(.*index.m3u8.*)$`)
	m := r.FindStringSubmatch(path)
	if len(m) < 3 {
		return "", "", false
	}
	slice := strings.Split(m[1], "+")
	pathTmpl := "/hls/%s/" + m[2]
	return slice[len(slice)-1], pathTmpl, true
}

func parsePlaybackIDJS(path string) (string, string, bool) {
	r := regexp.MustCompile(`^/json_([\w+-]+).js$`)
	m := r.FindStringSubmatch(path)
	if len(m) < 2 {
		return "", "", false
	}
	slice := strings.Split(m[1], "+")
	return slice[len(slice)-1], "/json_%s.js", true
}

func parsePlaybackID(path string) (string, string, bool) {
	parsers := []func(string) (string, string, bool){parsePlaybackIDHLS, parsePlaybackIDJS}
	for _, parser := range parsers {
		playbackID, suffix, isValid := parser(path)
		if isValid {
			return playbackID, suffix, isValid
		}
	}
	return "", "", false
}

func protocol(r *http.Request) string {
	if r.Header.Get("X-Forwarded-Proto") == "https" {
		return "https"
	}
	return "http"
}

func queryMistForClosestNode(playbackID, lat, lon, prefix string) (string, error) {
	// First, check to see if any server has this stream
	_, err1 := queryMistForClosestNodeSource(playbackID, lat, lon, prefix, true)
	// Then, check the best playback server
	node, err2 := queryMistForClosestNodeSource(playbackID, lat, lon, prefix, false)
	// If we can't get a playback server, error
	if err2 != nil {
		return "", err2
	}
	// If we didn't find the stream but we did find a node, return it with the error for 404s
	if err1 != nil {
		return node, err1
	}
	// Good path, we found the stream and a playback nodew!
	return node, nil
}

func queryMistForClosestNodeSource(playbackID, lat, lon, prefix string, source bool) (string, error) {
	if prefix != "" {
		prefix += "+"
	}
	var murl string
	enc := url.QueryEscape(fmt.Sprintf("%s%s", prefix, playbackID))
	if source {
		murl = fmt.Sprintf("http://localhost:%d/?source=%s", mistUtilLoadPort, enc)
	} else {
		murl = fmt.Sprintf("http://localhost:%d/%s", mistUtilLoadPort, enc)
	}
	glog.V(8).Infof("MistUtilLoad started request=%s", murl)
	req, err := http.NewRequest("GET", murl, nil)
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
		return "", fmt.Errorf("GET request '%s' failed with http status code %d", murl, resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("GET request '%s' failed while reading response body", murl)
	}
	glog.V(8).Infof("MistUtilLoad responded request=%s response=%s", murl, body)
	if string(body) == "FULL" {
		return "", fmt.Errorf("GET request '%s' returned 'FULL'", murl)
	}
	return string(body), nil
}
