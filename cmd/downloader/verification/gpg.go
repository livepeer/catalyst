package verification

import (
	"io/ioutil"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/livepeer/catalyst/cmd/downloader/constants"
	glog "github.com/magicsong/color-glog"
)

// VerifyGPGSignature raises an error if provided `.sig` file is not
// valid GPG signature for the given file.
func VerifyGPGSignature(fileName, signatureFileName string) error {
	// Generate keyring for the key fingerprint A2F9039A8603C44C21414432A2224D4537874DB2
	key, err := crypto.NewKeyFromArmored(constants.PGPPublicKey)
	if err != nil {
		glog.Error(err)
		return err
	}
	keyRing, err := crypto.NewKeyRing(key)
	if err != nil {
		glog.Error(err)
		return err
	}
	glog.V(7).Info("GPG keyring initialised. Proceeding to load signature now!")

	// Read GPG binary signature file
	reader, err := ioutil.ReadFile(signatureFileName)
	signature := crypto.NewPGPSignature(reader)
	glog.V(7).Info("GPG signature read success!")

	// Load signed file to memory as a binary message
	data, err := ioutil.ReadFile(fileName)
	message := crypto.NewPlainMessage(data)

	// Verification step
	err = keyRing.VerifyDetached(message, signature, crypto.GetUnixTime())
	if err != nil {
		glog.Errorf("GPG verification failed for %q with error %s", fileName, err)
		return err
	}
	glog.Info("GPG verification successful.")
	return nil
}
