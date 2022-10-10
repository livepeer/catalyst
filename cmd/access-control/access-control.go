package accesscontrol

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	glog "github.com/magicsong/color-glog"
)

type PlaybackAccessControl struct {
	gateURL string
	cache   map[string]map[string]*PlaybackAccessControlEntry
	*http.Client
}

type PlaybackAccessControlEntry struct {
	Stale  time.Time
	MaxAge time.Time
	Mutex  sync.Mutex
	Allow  bool
}

type PlaybackAccessControlRequest struct {
	Type   string `json:"type"`
	Pub    string `json:"pub"`
	Stream string `json:"stream"`
}

const UserNewTrigger = "USER_NEW"

func TriggerHandler(gateURL string) http.Handler {
	playbackAccessControl := PlaybackAccessControl{
		gateURL,
		make(map[string]map[string]*PlaybackAccessControlEntry),
		&http.Client{},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)

		if err != nil {
			w.Write([]byte("false"))
			glog.Errorf("Unable to parse trigger body %v", err)
			return
		}

		triggerName := r.Header.Get("X-Trigger")

		switch triggerName {
		case UserNewTrigger:
			w.Write(handleUserNew(&playbackAccessControl, payload))
			return
		default:
			w.Write([]byte("false"))
			glog.Errorf("Trigger not handled %v", triggerName)
			return
		}

	})
}

func handleUserNew(ac *PlaybackAccessControl, payload []byte) []byte {
	lines := strings.Split(string(payload), "\n")

	if len(lines) != 6 {
		glog.Errorf("Malformed trigger payload")
		return []byte("false")
	}

	requestURL, err := url.Parse(lines[4])
	if err != nil {
		glog.Errorf("Unable to parse URL %v", err)
		return []byte("false")
	}

	playbackID := lines[0]
	jwtToken := requestURL.Query().Get("jwt")

	var pubKey string
	if jwtToken != "" {
		claims, err := decodeJwt(jwtToken)
		if err != nil {
			glog.Errorf("Unable to decode JWT token %v", err)
			return []byte("false")
		}

		if playbackID != claims.Subject {
			glog.Errorf("PlaybackId mismatch")
			return []byte("false")
		}

		pubKey = claims.PublicKey
	}

	playbackAccessControl, err := getPlaybackAccessControlInfo(ac, playbackID, pubKey)
	if err != nil {
		glog.Errorf("Unable to get playback access control info %v", err)
		return []byte("false")
	}

	if playbackAccessControl[pubKey].Allow {
		return []byte("true")
	}

	return []byte("false")
}

func getPlaybackAccessControlInfo(ac *PlaybackAccessControl, playbackID, pubKey string) (map[string]*PlaybackAccessControlEntry, error) {
	if ac.cache[playbackID] == nil ||
		ac.cache[playbackID][pubKey] == nil ||
		time.Now().After(ac.cache[playbackID][pubKey].Stale) {

		err := cachePlaybackAccessControlInfo(ac, playbackID, pubKey)
		if err != nil {
			return nil, err
		}

	} else if time.Now().After(ac.cache[playbackID][pubKey].MaxAge) {
		ctx := context.TODO()
		go func() {
			ac.cache[playbackID][pubKey].Mutex.Lock()
			if time.Now().After(ac.cache[playbackID][pubKey].Stale) {
				cachePlaybackAccessControlInfo(ac, playbackID, pubKey)
			}
			ac.cache[playbackID][pubKey].Mutex.Unlock()
			ctx.Done()
		}()
	}

	return ac.cache[playbackID], nil
}

func cachePlaybackAccessControlInfo(ac *PlaybackAccessControl, playbackID, pubKey string) error {
	body, err := json.Marshal(PlaybackAccessControlRequest{"jwt", pubKey, playbackID})
	if err != nil {
		return err
	}

	allow, maxAge, stale, err := queryGate(ac, body)
	if err != nil {
		return err
	}

	var maxAgeTime = time.Now().Add(time.Duration(maxAge) * time.Second)
	var staleTime = time.Now().Add(time.Duration(stale) * time.Second)

	if ac.cache[playbackID] == nil {
		ac.cache[playbackID] = make(map[string]*PlaybackAccessControlEntry)
		ac.cache[playbackID][pubKey] = &PlaybackAccessControlEntry{staleTime, maxAgeTime, sync.Mutex{}, allow}
	} else {
		ac.cache[playbackID][pubKey].Allow = allow
		ac.cache[playbackID][pubKey].MaxAge = maxAgeTime
		ac.cache[playbackID][pubKey].Stale = staleTime
	}

	return nil
}

var queryGate = func(ac *PlaybackAccessControl, body []byte) (bool, int64, int64, error) {
	req, err := http.NewRequest("POST", ac.gateURL, bytes.NewReader(body))
	if err != nil {
		return false, 0, 0, err
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := ac.Client.Do(req)
	if err != nil {
		return false, 0, 0, err
	}

	defer res.Body.Close()

	cacheControlDirectives := strings.Split(res.Header.Get("Cache-Control"), ",")

	var maxAge, stale int64 = 0, 0
	if len(cacheControlDirectives) == 2 {
		maxAge, err = strconv.ParseInt(strings.Split(cacheControlDirectives[0], "=")[1], 10, 64)
		if err != nil {
			return res.StatusCode/100 == 2, 120, 600, nil
		}

		stale, err = strconv.ParseInt(strings.Split(cacheControlDirectives[1], "=")[1], 10, 64)
		if err != nil {
			return res.StatusCode/100 == 2, 120, 600, nil
		}
	}

	return res.StatusCode/100 == 2, maxAge, stale, nil
}

type PlaybackGateClaims struct {
	PublicKey string `json:"pub"`
	jwt.RegisteredClaims
}

func (c *PlaybackGateClaims) Valid() error {
	if err := c.RegisteredClaims.Valid(); err != nil {
		return err
	}
	if c.Subject == "" {
		return errors.New("missing sub claim")
	}
	if c.PublicKey == "" {
		return errors.New("missing pub claim")
	}
	if c.ExpiresAt == nil {
		return errors.New("missing exp claim")
	} else if time.Until(c.ExpiresAt.Time) > 7*24*time.Hour {
		return errors.New("exp claim too far in the future")
	}
	return nil
}

func decodeJwt(tokenString string) (*PlaybackGateClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &PlaybackGateClaims{}, func(token *jwt.Token) (interface{}, error) {
		pub := token.Claims.(*PlaybackGateClaims).PublicKey
		decodedPubkey, err := base64.StdEncoding.DecodeString(pub)
		if err != nil {
			return nil, err
		}

		return jwt.ParseECPublicKeyFromPEM(decodedPubkey)
	})

	if err != nil {
		return nil, err
	} else if err = token.Claims.Valid(); err != nil {
		return nil, err
	} else if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return token.Claims.(*PlaybackGateClaims), nil
}
