package parser

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestFromFile(t *testing.T) {
	tmpFile, err := os.CreateTemp(".", "Dockerfile")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	for _, s := range []string{"MAINTAINER", "Maintainer", "maintainer"} {
		tmpFile.WriteString(s + " author@example.com")
		Dockerfile, err := FromFile(tmpFile.Name())

		require.NoError(t, err)
		require.Equal(t, Dockerfile.AllInstructions[0].Name, "MAINTAINER")
		for _, inst := range Dockerfile.AllInstructions {
			require.Equal(t, inst.IsValid, true)
			require.Equal(t, inst.Errors, []string(nil))
		}
	}
}
