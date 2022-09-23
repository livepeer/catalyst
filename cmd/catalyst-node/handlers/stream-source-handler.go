package handlers

import (
	"fmt"
	"io"
	"net/http"

	serfclient "github.com/hashicorp/serf/client"
	glog "github.com/magicsong/color-glog"
)

func StreamSourceHandler(lat, lon float64, serfClient *serfclient.RPCClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			glog.Errorf("error handling STREAM_SOURCE body=%s", err)
			w.Write([]byte("push://"))
			return
		}
		streamName := string(b)
		glog.V(7).Infof("got mist STREAM_SOURCE request=%s", streamName)
		latStr := fmt.Sprintf("%f", lat)
		lonStr := fmt.Sprintf("%f", lon)
		dtscURL, err := queryMistForClosestNodeSource(streamName, latStr, lonStr, "", true)
		if err != nil {
			glog.Errorf("error querying mist for STREAM_SOURCE: %s", err)
			w.Write([]byte("push://"))
			return
		}
		outURL, err := resolveNodeURL(dtscURL, serfClient)
		if err != nil {
			glog.Errorf("error finding STREAM_SOURCE: %s", err)
			w.Write([]byte("push://"))
			return
		}
		glog.V(7).Infof("replying to Mist STREAM_SOURCE request=%s response=%s", streamName, outURL)
		w.Write([]byte(outURL))
	})
}
