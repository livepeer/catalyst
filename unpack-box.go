package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"net/http"

	"golang.org/x/crypto/pbkdf2"
)

func main() {
	url := "http://localhost:8080/api/user"
	fmt.Println("URL:>", url)

	dk := pbkdf2.Key([]byte("password"), []byte(`69195A9476F08546`), 4096, 10000, sha256.New)
	fmt.Println(dk)

	var jsonStr = []byte(`{"email":"support@livepeer.com", "password": "password"}`)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	req.Header.Set("X-Custom-Header", "myvalue")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("response Body:", string(body))
}
