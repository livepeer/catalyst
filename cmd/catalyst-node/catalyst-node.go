package main

import (
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"syscall"

	serfclient "github.com/hashicorp/serf/client"
	"github.com/livepeer/catalyst/cmd/catalyst-node/balancer"
	"github.com/livepeer/catalyst/cmd/catalyst-node/cluster"
	accesscontrol "github.com/livepeer/catalyst/cmd/catalyst-node/handlers/access-control"
	"github.com/livepeer/livepeer-data/pkg/mistconnector"
	glog "github.com/magicsong/color-glog"
	"github.com/peterbourgon/ff/v3"
)

const (
	httpPort         = 8090
	httpInternalPort = 8091
)

var Version = "unknown"
var mistUtilLoadPort = rand.Intn(10000) + 40000

type Node struct {
	Cluster  cluster.ClusterIface
	Balancer balancer.BalancerIface
	Config   *Config
}

type Config struct {
	serfRPCAddress           string
	serfRPCAuthKey           string
	serfTags                 map[string]string
	mistLoadBalancerPort     int
	mistLoadBalancerTemplate string
	Verbosity                int
	MistJSON                 bool
	Version                  bool
	BalancerArgs             string
	HTTPAddress              string
	HTTPInternalAddress      string
	RedirectPrefixes         []string
	NodeHost                 string
	NodeLatitude             float64
	NodeLongitude            float64
	GateURL                  string
}

func parseSerfConfig(config *cluster.Config, retryJoin, serfTags *string) {
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

func main() {
	var config = &Config{}

	flag.Set("logtostderr", "true")
	vFlag := flag.Lookup("v")
	fs := flag.NewFlagSet("catalyst-node-connected", flag.ExitOnError)

	fs.BoolVar(&config.MistJSON, "j", false, "Print application info as json")
	fs.IntVar(&config.Verbosity, "v", 3, "Log verbosity.  {4|5|6}")
	fs.BoolVar(&config.Version, "version", false, "Print out the version")
	fs.StringVar(&config.BalancerArgs, "balancer-args", "", "arguments passed to MistUtilLoad")
	fs.StringVar(&config.NodeHost, "node-host", "", "Hostname this node should handle requests for. Requests on any other domain will trigger a redirect. Useful as a 404 handler to send users to another node.")
	fs.Float64Var(&config.NodeLatitude, "node-latitude", 0, "Latitude of this Catalyst node. Used for load balancing.")
	fs.Float64Var(&config.NodeLongitude, "node-longitude", 0, "Longitude of this Catalyst node. Used for load balancing.")
	prefixes := fs.String("redirect-prefixes", "", "Set of valid prefixes of playback id which are handled by mistserver")

	// Catalyst web server
	fs.StringVar(&config.HTTPAddress, "http-addr", fmt.Sprintf("127.0.0.1:%d", httpPort), "Address to bind for external-facing Catalyst HTTP handling")
	fs.StringVar(&config.HTTPInternalAddress, "http-internal-addr", fmt.Sprintf("127.0.0.1:%d", httpInternalPort), "Address to bind for internal privileged HTTP commands")

	fs.StringVar(&config.serfRPCAddress, "serf-rpc-address", "127.0.0.1:7373", "Serf RPC address")
	fs.StringVar(&config.serfRPCAuthKey, "serf-rpc-auth-key", "", "Serf RPC auth key")
	serfTags := fs.String("serf-tags", "node=media", "Serf tags for Catalyst nodes")
	fs.IntVar(&config.mistLoadBalancerPort, "mist-load-balancer-port", mistUtilLoadPort, "MistUtilLoad port (default random)")
	fs.StringVar(&config.mistLoadBalancerTemplate, "mist-load-balancer-template", "http://%s:4242", "template for specifying the host that should be queried for Prometheus stat output for this node")

	// Serf commands passed straight through to the agent
	clusterConfig := cluster.Config{}
	fs.StringVar(&clusterConfig.BindAddr, "bind", "0.0.0.0:9935", "Address to bind network listeners to. To use an IPv6 address, specify [::1] or [::1]:7946.")
	fs.StringVar(&clusterConfig.AdvertiseAddr, "advertise", "0.0.0.0", "Address to advertise to the other cluster members")
	fs.StringVar(&clusterConfig.RPCAddr, "rpc-addr", "127.0.0.1:7373", "Address to bind the RPC listener.")
	retryJoin := fs.String("retry-join", "", "An agent to join with. This flag be specified multiple times. Does not exit on failure like -join, used to retry until success.")
	fs.StringVar(&clusterConfig.EncryptKey, "encrypt", "", "Key for encrypting network traffic within Serf. Must be a base64-encoded 32-byte key.")
	fs.StringVar(&clusterConfig.Profile, "profile", "", "Profile is used to control the timing profiles used in Serf. The default if not provided is wan.")
	fs.StringVar(&clusterConfig.NodeName, "node", "", "Name of this node. Must be unique in the cluster")

	// Playback gating Api
	fs.StringVar(&config.GateURL, "gate-url", "http://localhost:3004/api/access-control/gate", "Address to contact playback gating API for access control verification")

	ff.Parse(
		fs, os.Args[1:],
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
		ff.WithEnvVarPrefix("CATALYST_NODE"),
	)
	vFlag.Value.Set(fmt.Sprint(config.Verbosity))
	flag.CommandLine.Parse(nil)

	if config.MistJSON {
		mistconnector.PrintMistConfigJson(
			"catalyst-node",
			"Catalyst multi-node server. Coordinates stream replication and load balancing to multiple catalyst nodes.",
			"Catalyst Node",
			Version,
			fs,
		)
		return
	}

	if config.Version {
		fmt.Println("catalyst-node version: " + Version)
		fmt.Printf("golang runtime version: %s %s\n", runtime.Compiler, runtime.Version())
		fmt.Printf("architecture: %s\n", runtime.GOARCH)
		fmt.Printf("operating system: %s\n", runtime.GOOS)
		return
	}

	// Handle converting CLI flags into correct format
	config.RedirectPrefixes = strings.Split(*prefixes, ",")
	glog.V(4).Infof("found redirectPrefixes=%v", config.RedirectPrefixes)
	parseSerfConfig(&clusterConfig, retryJoin, serfTags)
	balancerArgs := strings.Split(config.BalancerArgs, " ")

	// Create main node object
	n := &Node{
		Config: config,
	}

	// Start cluster
	clusterConfig.SerfRPCAddress = config.serfRPCAddress
	clusterConfig.SerfRPCAuthKey = config.serfRPCAuthKey
	memberChan := make(chan *[]serfclient.Member)
	clusterConfig.MemberChan = memberChan
	n.Cluster = cluster.NewCluster(&clusterConfig)
	go func() {
		err := n.Cluster.Start()
		// TODO: graceful shutdown upon error
		n.Balancer.Kill()
		panic(fmt.Errorf("error in cluster connection: %w", err))
	}()

	// Start balancer
	n.Balancer = balancer.NewBalancer(&balancer.Config{
		Args:                     balancerArgs,
		MistUtilLoadPort:         uint32(config.mistLoadBalancerPort),
		MistLoadBalancerTemplate: config.mistLoadBalancerTemplate,
	})
	go func() {
		err := n.Balancer.Start()
		if err != nil {
			glog.Fatal(err)
		}
	}()

	// Start HTTP servers
	go n.startCatalystWebServer()
	go n.startInternalWebServer()

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
		for {
			s := <-c
			glog.Errorf("caught signal=%v killing MistUtilLoad", s)
			n.Balancer.Kill()
		}
	}()

	// Start main reconcillation loop - get members from serf and update MistUtilLoad
	for {
		members := <-memberChan
		n.Balancer.UpdateMembers(members)
	}
}

func (n *Node) startCatalystWebServer() {
	http.Handle("/", n.redirectHandler(n.Config.RedirectPrefixes, n.Config.NodeHost))
	http.Handle("/triggers", accesscontrol.TriggerHandler(n.Config.GateURL))
	glog.Infof("HTTP server listening on %s", n.Config.HTTPAddress)
	glog.Fatal(http.ListenAndServe(n.Config.HTTPAddress, nil))
}

func (n *Node) startInternalWebServer() {
	http.Handle("/STREAM_SOURCE", n.streamSourceHandler(n.Config.NodeLatitude, n.Config.NodeLongitude))
	glog.Infof("Internal HTTP server listening on %s", n.Config.HTTPInternalAddress)
	glog.Fatal(http.ListenAndServe(n.Config.HTTPInternalAddress, nil))
}

func (n *Node) redirectHandler(redirectPrefixes []string, nodeHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")

		if nodeHost != "" {
			host := r.Host
			if host != nodeHost {
				newURL, err := url.Parse(r.URL.String())
				if err != nil {
					glog.Errorf("failed to parse incoming url for redirect url=%s err=%s", r.URL.String(), err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				newURL.Scheme = protocol(r)
				newURL.Host = nodeHost
				http.Redirect(w, r, newURL.String(), http.StatusFound)
				glog.V(6).Infof("NodeHost redirect host=%s nodeHost=%s from=%s to=%s", host, nodeHost, r.URL, newURL)
				return
			}
		}

		prefix, playbackID, pathTmpl, isValid := parsePlaybackID(r.URL.Path)
		if !isValid {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		lat := r.Header.Get("X-Latitude")
		lon := r.Header.Get("X-Longitude")

		bestNode, fullPlaybackID, err := n.Balancer.GetBestNode(redirectPrefixes, playbackID, lat, lon, prefix)
		if err != nil {
			glog.Errorf("failed to find either origin or fallback server for playbackID=%s err=%s", playbackID, err)
			w.WriteHeader(http.StatusBadGateway)
			return
		}

		rPath := fmt.Sprintf(pathTmpl, fullPlaybackID)
		rURL := fmt.Sprintf("%s://%s%s?%s", protocol(r), bestNode, rPath, r.URL.RawQuery)
		rURL, err = n.resolveNodeURL(rURL)
		if err != nil {
			glog.Errorf("failed to resolve node URL playbackID=%s err=%s", playbackID, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		glog.V(6).Infof("generated redirect url=%s", rURL)
		http.Redirect(w, r, rURL, http.StatusFound)
	})
}

func (n *Node) streamSourceHandler(lat, lon float64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Workaround for https://github.com/DDVTECH/mistserver/issues/114
		w.Header().Set("Transfer-Encoding", "chunked")
		b, err := io.ReadAll(r.Body)
		if err != nil {
			glog.Errorf("error handling STREAM_SOURCE body=%s", err)
			w.Write([]byte("push://"))
			return
		}
		streamName := string(b)
		glog.V(7).Infof("got mist STREAM_SOURCE request=%s", streamName)

		// if VOD source is detected, return empty response to use input URL as configured
		if strings.HasPrefix(streamName, "catalyst_vod_") || strings.HasPrefix(streamName, "tr_src_") {
			w.Write([]byte(""))
			return
		}

		latStr := fmt.Sprintf("%f", lat)
		lonStr := fmt.Sprintf("%f", lon)
		dtscURL, err := n.Balancer.QueryMistForClosestNodeSource(streamName, latStr, lonStr, "", true)
		if err != nil {
			glog.Errorf("error querying mist for STREAM_SOURCE: %s", err)
			w.Write([]byte("push://"))
			return
		}
		outURL, err := n.resolveNodeURL(dtscURL)
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
func (n *Node) resolveNodeURL(streamURL string) (string, error) {
	u, err := url.Parse(streamURL)
	if err != nil {
		return "", err
	}
	nodeName := u.Host
	protocol := u.Scheme

	member, err := n.Cluster.Member(map[string]string{}, "alive", nodeName)
	if err != nil {
		return "", err
	}
	addr, has := member.Tags[protocol]
	if !has {
		glog.V(7).Infof("no tag found, not tag resolving protocol=%s nodeName=%s", protocol, nodeName)
		return streamURL, nil
	}
	u2, err := url.Parse(addr)
	if err != nil {
		err = fmt.Errorf("node has unparsable tag!! nodeName=%s protocol=%s tag=%s", nodeName, protocol, addr)
		glog.Error(err)
		return "", err
	}
	u2.Path = u.Path
	u2.RawQuery = u.RawQuery
	return u2.String(), nil
}

func parsePlus(plusString string) (string, string) {
	slice := strings.Split(plusString, "+")
	prefix := ""
	playbackID := ""
	if len(slice) > 2 {
		return "", ""
	}
	if len(slice) == 2 {
		prefix = slice[0]
		playbackID = slice[1]
	} else {
		playbackID = slice[0]
	}
	return prefix, playbackID
}

// Incoming requests might come with some prefix attached to the
// playback ID. We try to drop that here by splitting at `+` and
// picking the last piece. For eg.
// incoming path = '/hls/video+4712oox4msvs9qsf/index.m3u8'
// playbackID = '4712oox4msvs9qsf'
func parsePlaybackIDHLS(path string) (string, string, string, bool) {
	r := regexp.MustCompile(`^/hls/([\w+-]+)/(.*index.m3u8.*)$`)
	m := r.FindStringSubmatch(path)
	if len(m) < 3 {
		return "", "", "", false
	}
	prefix, playbackID := parsePlus(m[1])
	if playbackID == "" {
		return "", "", "", false
	}
	pathTmpl := "/hls/%s/" + m[2]
	return prefix, playbackID, pathTmpl, true
}

func parsePlaybackIDJS(path string) (string, string, string, bool) {
	r := regexp.MustCompile(`^/json_([\w+-]+).js$`)
	m := r.FindStringSubmatch(path)
	if len(m) < 2 {
		return "", "", "", false
	}
	prefix, playbackID := parsePlus(m[1])
	if playbackID == "" {
		return "", "", "", false
	}
	return prefix, playbackID, "/json_%s.js", true
}

func parsePlaybackID(path string) (string, string, string, bool) {
	parsers := []func(string) (string, string, string, bool){parsePlaybackIDHLS, parsePlaybackIDJS}
	for _, parser := range parsers {
		prefix, playbackID, suffix, isValid := parser(path)
		if isValid {
			return prefix, playbackID, suffix, isValid
		}
	}
	return "", "", "", false
}

func protocol(r *http.Request) string {
	if r.Header.Get("X-Forwarded-Proto") == "https" {
		return "https"
	}
	return "http"
}
