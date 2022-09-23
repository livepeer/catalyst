package handlers

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"

	serfclient "github.com/hashicorp/serf/client"
	glog "github.com/magicsong/color-glog"
)

var mistUtilLoadPort = rand.Intn(10000) + 40000
var getClosestNode = queryMistForClosestNode

func RedirectHandler(redirectPrefixes []string, nodeHost string, serfClient *serfclient.RPCClient) http.Handler {
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
		rURL, err = resolveNodeURL(rURL, serfClient)
		if err != nil {
			glog.Errorf("failed to resolve node URL playbackID=%s err=%s", playbackID, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		glog.V(6).Infof("generated redirect url=%s", rURL)
		http.Redirect(w, r, rURL, http.StatusFound)
	})
}

// Given a dtsc:// or https:// url, resolve the proper address of the node via serf tags
func resolveNodeURL(streamURL string, serfClient *serfclient.RPCClient) (string, error) {
	u, err := url.Parse(streamURL)
	if err != nil {
		return "", err
	}
	nodeName := u.Host
	protocol := u.Scheme

	member, err := getSerfMember(nodeName, serfClient)
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

func querySerfForMember(name string, serfClient *serfclient.RPCClient) (*serfclient.Member, error) {
	members, err := serfClient.MembersFiltered(map[string]string{}, "alive", name)
	if err != nil {
		return nil, err
	}
	if len(members) < 1 {
		return nil, fmt.Errorf("could not find serf member name=%s", name)
	}
	if len(members) > 1 {
		glog.Errorf("found multiple serf members with the same name! this shouldn't happen! name=%s count=%d", name, len(members))
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("GET request '%s' failed while reading response body", murl)
	}
	glog.V(8).Infof("MistUtilLoad responded request=%s response=%s", murl, body)
	if string(body) == "FULL" {
		return "", fmt.Errorf("GET request '%s' returned 'FULL'", murl)
	}
	return string(body), nil
}
