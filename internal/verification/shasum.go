package verification

import (
	"os/exec"

	"github.com/golang/glog"
)

// VerifySHA256Digest verifies the sha256 digest by relying on
// `shasum` command present on the system instead of computing the
// same itself.
func VerifySHA256Digest(directory, checksumFile string) error {
	glog.Infof("Verifying shasum with file=%s", checksumFile)
	cmd := exec.Command("shasum", "--algorithm", "256", "--ignore-missing", "--check", checksumFile)
	// Run `shasum` from same directory.
	cmd.Dir = directory
	output, err := cmd.Output()
	if err != nil {
		glog.Error("Failed to run shasum")
		return err
	}
	glog.Info(string(output))
	return nil
}
