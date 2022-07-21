package verification

import (
	"os"
	"testing"
)

func TestSignaureValid(t *testing.T) {
	path, err := os.Getwd()
	t.Log(path)
	err = VerifyGPGSignature(path+"/../../livepeer-api-linux-amd64.tar.gz", path+"/../../livepeer-api-linux-amd64.tar.gz.sig")
	t.Log(err)
}
