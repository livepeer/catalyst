package config

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestItCanPassthroughEmptyConfig(t *testing.T) {
	generated, err := Config()
	require.NoError(t, err)
	require.Empty(t, jsonEQ(fullstack, generated))
}

// returns empty if equal
func jsonEQ(x, y []byte) string {
	return cmp.Diff(x, y, cmp.Transformer("ParseJSON", func(in []byte) (out any) {
		if err := json.Unmarshal(in, &out); err != nil {
			return err
		}
		return out
	}))

}
