package accesscontrol

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	glog "github.com/magicsong/color-glog"
	"github.com/pquerna/cachecontrol/cacheobject"
)

type PlaybackAccessControl struct {
	gateURL string
	*http.Client
	cache map[string]map[string]*PlaybackAccessControlEntry
	mutex sync.RWMutex
}

type PlaybackAccessControlEntry struct {
	Stale  time.Time
	MaxAge time.Time
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
		&http.Client{},
		make(map[string]map[string]*PlaybackAccessControlEntry),
		sync.RWMutex{},
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
			glog.Errorf("Unable to decode on incoming playbackId %v with jwt %v", playbackID, jwtToken)
			return []byte("false")
		}

		if playbackID != claims.Subject {
			glog.Errorf("PlaybackId mismatch playbackId=%v != claimed=%v", playbackID, claims.Subject)
			return []byte("false")
		}

		glog.Infof("Access control request for playbackId %v with pubkey %v", playbackID, claims.PublicKey)

		pubKey = claims.PublicKey
	}

	playbackAccessControlAllowed, err := getPlaybackAccessControlInfo(ac, playbackID, pubKey)
	if err != nil {
		glog.Errorf("Unable to get playback access control info for playbackId %v with pubkey %v", playbackID, pubKey)
		return []byte("false")
	}

	if playbackAccessControlAllowed {
		glog.Infof("Playback access control allowed for playbackId %v with pubkey %v", playbackID, pubKey)
		return []byte("true")
	}

	glog.Infof("Playback access control denied for playbackId %v", playbackID)
	return []byte("false")
}

func getPlaybackAccessControlInfo(ac *PlaybackAccessControl, playbackID, pubKey string) (bool, error) {
	ac.mutex.RLock()
	entry := ac.cache[playbackID][pubKey]
	ac.mutex.RUnlock()

	if isStale(entry) {
		glog.Infof("Cache stale for playbackId %v with pubkey %v", playbackID, pubKey)
		err := cachePlaybackAccessControlInfo(ac, playbackID, pubKey)
		if err != nil {
			return false, err
		}
	} else if time.Now().After(entry.MaxAge) {
		glog.Infof("Cache expired for playbackId %v with pubkey %v", playbackID, pubKey)
		go func() {
			ac.mutex.RLock()
			stillStale := isStale(ac.cache[playbackID][pubKey])
			ac.mutex.RUnlock()
			if stillStale {
				cachePlaybackAccessControlInfo(ac, playbackID, pubKey)
			}
		}()
	} else {
		glog.Infof("Cache hit for playbackId %v with pubkey %v", playbackID, pubKey)
	}

	ac.mutex.RLock()
	entry = ac.cache[playbackID][pubKey]
	ac.mutex.RUnlock()

	glog.Infof("playbackId %v with pubkey %v playback allowed=%v", playbackID, pubKey, entry.Allow)

	return entry.Allow, nil
}

func isStale(entry *PlaybackAccessControlEntry) bool {
	return entry == nil || time.Now().After(entry.Stale)
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
	ac.mutex.Lock()
	defer ac.mutex.Unlock()
	if ac.cache[playbackID] == nil {
		ac.cache[playbackID] = make(map[string]*PlaybackAccessControlEntry)
	}
	ac.cache[playbackID][pubKey] = &PlaybackAccessControlEntry{staleTime, maxAgeTime, allow}
	return nil
}

var queryGate = func(ac *PlaybackAccessControl, body []byte) (bool, int32, int32, error) {
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
	cc, err := cacheobject.ParseResponseCacheControl(res.Header.Get("Cache-Control"))
	if err != nil {
		return false, 0, 0, err
	}

	return res.StatusCode/100 == 2, int32(cc.MaxAge), int32(cc.StaleWhileRevalidate), nil
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
		glog.Errorf("Unable to parse jwt token %v", err)
		return nil, err
	} else if err = token.Claims.Valid(); err != nil {
		glog.Errorf("Invalid claims: %v", err)
		return nil, err
	} else if !token.Valid {
		glog.Errorf("Invalid token %v for playbackId %v", tokenString, token.Claims.(*PlaybackGateClaims).Subject)
		return nil, errors.New("invalid token")
	}
	return token.Claims.(*PlaybackGateClaims), nil
}
