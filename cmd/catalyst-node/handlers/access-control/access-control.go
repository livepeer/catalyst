package accesscontrol

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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
	cache      map[string]map[string]*PlaybackAccessControlEntry
	mutex      sync.RWMutex
	gateClient GateAPICaller
}

type PlaybackAccessControlEntry struct {
	Stale  time.Time
	MaxAge time.Time
	Allow  bool
}

type PlaybackAccessControlRequest struct {
	Type      string `json:"type"`
	Pub       string `json:"pub"`
	AccessKey string `json:"accessKey"`
	Stream    string `json:"stream"`
}

type GateAPICaller interface {
	QueryGate(body []byte) (bool, int32, int32, error)
}

type GateClient struct {
	Client  *http.Client
	gateURL string
}

const UserNewTrigger = "USER_NEW"

func NewPlaybackAccessControl(gateURL string) *PlaybackAccessControl {
	return &PlaybackAccessControl{
		cache: make(map[string]map[string]*PlaybackAccessControlEntry),
		gateClient: &GateClient{
			gateURL: gateURL,
			Client:  &http.Client{},
		},
	}
}

func (ac *PlaybackAccessControl) TriggerHandler() http.Handler {
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
			w.Write(ac.handleUserNew(payload))
			return
		default:
			w.Write([]byte("false"))
			glog.Errorf("Trigger not handled %v", triggerName)
			return
		}

	})
}

func (ac *PlaybackAccessControl) handleUserNew(payload []byte) []byte {
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
	playbackID = playbackID[strings.Index(playbackID, "+")+1:]

	playbackAccessControlAllowed, err := ac.IsAuthorized(playbackID, requestURL)
	if err != nil {
		glog.Errorf("Unable to get playback access control info for playbackId=%v err=%s", playbackID, err.Error())
		return []byte("false")
	}

	if playbackAccessControlAllowed {
		glog.Infof("Playback access control allowed for playbackId=%v", playbackID)
		return []byte("true")
	}

	glog.Infof("Playback access control denied for playbackId=%v", playbackID)
	return []byte("false")
}

func (ac *PlaybackAccessControl) IsAuthorized(playbackID string, reqURL *url.URL) (bool, error) {
	acReq := PlaybackAccessControlRequest{Stream: playbackID}
	cacheKey := ""
	accessKey := reqURL.Query().Get("accessKey")
	jwt := reqURL.Query().Get("jwt")
	if accessKey != "" {
		acReq.Type = "accessKey"
		acReq.AccessKey = accessKey
		cacheKey = "accessKey_" + accessKey
	} else if jwt != "" {
		acReq.Pub = extractKeyFromJwt(jwt, acReq.Stream)
		if acReq.Pub == "" {
			return false, fmt.Errorf("failed to extract key from jwt: %s", jwt)
		}

		acReq.Type = "jwt"
		cacheKey = "jwtPubKey_" + acReq.Pub
	}

	body, err := json.Marshal(acReq)
	if err != nil {
		glog.Errorf("Unable to get playback access control info, JSON marshalling failed. playbackId=%v", acReq.Stream)
		return false, nil
	}

	return ac.GetPlaybackAccessControlInfo(acReq.Stream, cacheKey, body)
}

func (ac *PlaybackAccessControl) GetPlaybackAccessControlInfo(playbackID, cacheKey string, requestBody []byte) (bool, error) {
	ac.mutex.RLock()
	entry := ac.cache[playbackID][cacheKey]
	ac.mutex.RUnlock()

	if isExpired(entry) {
		glog.Infof("Cache expired for playbackId=%v cacheKey=%v", playbackID, cacheKey)
		err := ac.cachePlaybackAccessControlInfo(playbackID, cacheKey, requestBody)
		if err != nil {
			return false, err
		}
	} else if isStale(entry) {
		glog.Infof("Cache stale for playbackId=%v cacheKey=%v\n", playbackID, cacheKey)
		go func() {
			ac.mutex.RLock()
			stillStale := isStale(ac.cache[playbackID][cacheKey])
			ac.mutex.RUnlock()
			if stillStale {
				ac.cachePlaybackAccessControlInfo(playbackID, cacheKey, requestBody)
			}
		}()
	} else {
		glog.Infof("Cache hit for playbackId=%v cacheKey=%v", playbackID, cacheKey)
	}

	ac.mutex.RLock()
	entry = ac.cache[playbackID][cacheKey]
	ac.mutex.RUnlock()

	glog.Infof("playbackId=%v cacheKey=%v playback allowed=%v", playbackID, cacheKey, entry.Allow)

	return entry.Allow, nil
}

func isExpired(entry *PlaybackAccessControlEntry) bool {
	return entry == nil || time.Now().After(entry.Stale)
}

func isStale(entry *PlaybackAccessControlEntry) bool {
	return entry != nil && time.Now().After(entry.MaxAge) && !isExpired(entry)
}

func (ac *PlaybackAccessControl) cachePlaybackAccessControlInfo(playbackID, cacheKey string, requestBody []byte) error {
	allow, maxAge, stale, err := ac.gateClient.QueryGate(requestBody)
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
	ac.cache[playbackID][cacheKey] = &PlaybackAccessControlEntry{staleTime, maxAgeTime, allow}
	return nil
}

func (g *GateClient) QueryGate(body []byte) (bool, int32, int32, error) {
	req, err := http.NewRequest("POST", g.gateURL, bytes.NewReader(body))
	if err != nil {
		return false, 0, 0, err
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := g.Client.Do(req)
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
		glog.Errorf("Invalid registered claims %v", err)
		return err
	}
	if c.Subject == "" {
		glog.Errorf("Missing subject claim for playbackId=%v", c.Subject)
		return errors.New("missing sub claim")
	}
	if c.PublicKey == "" {
		glog.Infof("Missing pub claim for playbackId=%v", c.Subject)
		return errors.New("missing pub claim")
	}
	if c.ExpiresAt == nil {
		return errors.New("missing exp claim")
	} else if time.Until(c.ExpiresAt.Time) > 7*24*time.Hour {
		glog.Errorf("exp claim is too far in the future for playbackId=%v", c.Subject)
		return errors.New("exp claim too far in the future")
	}
	return nil
}

func extractKeyFromJwt(tokenString, playbackID string) string {
	claims, err := decodeJwt(tokenString)
	if err != nil {
		glog.Errorf("Unable to decode on incoming playbackId=%v jwt=%v", playbackID, tokenString)
		return ""
	}

	if playbackID != claims.Subject {
		glog.Errorf("PlaybackId mismatch playbackId=%v != claimed=%v", playbackID, claims.Subject)
		return ""
	}

	glog.Infof("Access control request for playbackId=%v pubkey=%v", playbackID, claims.PublicKey)
	return claims.PublicKey
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
		glog.Errorf("Invalid token=%v for playbackId=%v", tokenString, token.Claims.(*PlaybackGateClaims).Subject)
		return nil, errors.New("invalid token")
	}
	return token.Claims.(*PlaybackGateClaims), nil
}
