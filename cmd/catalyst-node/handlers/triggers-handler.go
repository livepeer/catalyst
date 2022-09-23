package handlers

import (
	"io"
	"net/http"
	"strings"

	glog "github.com/magicsong/color-glog"
)

const TriggerUserNew = "USER_NEW"

func TriggerHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)

		if err != nil {
			w.Write([]byte("false"))
			glog.Errorf("error %v", err)
			return
		}

		triggerName := r.Header.Get("X-Trigger")

		switch triggerName {
		case TriggerUserNew:
			w.Write(handleUserNew(payload))
			return
		default:
			w.Write([]byte("false"))
			glog.Errorf("default %v", err)
			return
		}

	})
}

func handleUserNew(payload []byte) []byte {
	lines := strings.Split(string(payload), "\n")

	if len(lines) != 6 {
		return []byte("false")
	}

	streamName := lines[0]
	connectionAddress := lines[1]
	connectionID := lines[2]
	connector := lines[3]
	requestURL := lines[4]
	sessionID := lines[5]

	glog.Infof("streamName: %v, connectionAddress: %v, connectionId: %v, connector: %v, requestUrl: %v, sessionId: %v",
		streamName, connectionAddress, connectionID, connector, requestURL, sessionID)

	return []byte("true")
}
