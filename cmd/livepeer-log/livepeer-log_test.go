package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLongOutput(t *testing.T) {
	testPath, err := filepath.Abs("log_spam_test.sh")
	require.NoError(t, err)
	c, b := exec.Command("go", "run", "livepeer-log.go", testPath), new(strings.Builder)
	c.Stderr = b
	err = c.Run()
	require.NoError(t, err)
	require.True(t, strings.Contains(b.String(), "success"))
}
