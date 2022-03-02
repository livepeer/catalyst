package gpg

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/livepeer/livepeer-in-a-box/internal/constants"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
)

// Taken from gist: https://gist.github.com/FZambia/f91ddffb1a2b776d56e1988c6048e4d8
func checkSign(fileName, signatureFileName string) error {
	fileContent, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}

	// Get a Reader for the signature file
	sigFile, err := os.Open(signatureFileName)
	if err != nil {
		return err
	}

	defer func() {
		if err := sigFile.Close(); err != nil {
			panic(err)
		}
	}()

	// Decode armored signature file
	block, err := armor.Decode(sigFile)
	if err != nil {
		return fmt.Errorf("error decoding signature file: %s", err)
	}
	if block.Type != "PGP SIGNATURE" {
		return errors.New("not an armored signature")
	}

	// Read signature body
	pack, err := packet.Read(block.Body)
	if err != nil {
		return fmt.Errorf("error reading signature file: %s", err)
	}

	// Was it really a signature file ? If yes, get the Signature
	signature, ok := pack.(*packet.Signature)
	if !ok {
		return errors.New("not a valid signature file")
	}

	// Decode armored public key
	block, err = armor.Decode(bytes.NewReader([]byte(constants.PGP_PUBLIC_KEY)))
	if err != nil {
		return fmt.Errorf("error decoding public key: %s", err)
	}
	if block.Type != "PGP PUBLIC KEY BLOCK" {
		return errors.New("not an armored public key")
	}

	// Read the key
	pack, err = packet.Read(block.Body)
	if err != nil {
		return fmt.Errorf("error reading public key: %s", err)
	}

	// Was it really a public key file ? If yes, get the PublicKey
	publicKey, ok := pack.(*packet.PublicKey)
	if !ok {
		return errors.New("invalid public key")
	}

	// Get the hash method used for the signature
	hash := signature.Hash.New()

	// Hash the content of the file (if the file is big, that's where you have to change the code to avoid getting the whole file in memory, by reading and writting in small chunks)
	_, err = hash.Write(fileContent)
	if err != nil {
		return err
	}

	// Check the signature
	err = publicKey.VerifySignature(hash, signature)
	if err != nil {
		return err
	}

	return nil
}
