package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

func main() {
	url := "http://10.173.0.1:8080/api/user"
	fmt.Println("URL:>", url)

	salt, _ := hex.DecodeString("69195A9476F08546")
	dk := pbkdf2.Key([]byte("password"), salt, 10000, 32, sha256.New)
	hash := strings.ToUpper(hex.EncodeToString(dk))
	var body = map[string]string{
		"email":    "admin@livepeer.dev",
		"password": hash,
	}
	b, _ := json.Marshal(body)
	fmt.Println(string(b))

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(b))

	client := &http.Client{}
	req.Header.Add("Content-Type", `application/json`)
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	res, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("response Body:", string(res))
}
