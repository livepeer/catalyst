package verification

import (
	"os/exec"

	glog "github.com/magicsong/color-glog"
)

// VerifySHA256Digest verifies the sha256 digest by relying on
// `shasum` command present on the system instead of computing the
// same itself.
func VerifySHA256Digest(directory, checksumFile string) error {
	cmd := exec.Command("shasum", "--algorithm", "256", "--ignore-missing", "--check", checksumFile)
	// Run `shasum` from same directory.
	cmd.Dir = directory
	output, err := cmd.CombinedOutput()
	glog.Infof("shasum output: %s", string(output))
	if err != nil {
		glog.Errorf("Failed to run shasum for %s", checksumFile)
		return err
	}
	return nil
}
