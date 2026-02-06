package linter

import (
	"github.com/mintoolkit/mint/pkg/docker/dockerfile/parser"
	"github.com/mintoolkit/mint/pkg/docker/linter/check"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestExecute(t *testing.T) {
	tmpFile, err := os.CreateTemp(".", "Dockerfile")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString(`FROM scratch as final
MAINTAINER author@example.com
LABEL stage=unittest
ADD . .
COPY . .
ENV
ARG
EXPOSE
RUN
HEALTHCHECK
ONBUILD
SHELL
ENTRYPOINT ["/bin/sh"]
VOLUME
WORKDIR
USER
STOPSIGNAL
CMD ["/bin/sh", "sleep"]
`)
	Dockerfile, err := parser.FromFile(tmpFile.Name())

	require.NoError(t, err)
	require.Equal(t, len(Dockerfile.AllInstructions), 18)

	options := Options{
		tmpFile.Name(),
		Dockerfile,
		true,
		".",
		true,
		nil,
		CheckSelector{},
		map[string]*check.Options{},
	}

	report, err := Execute(options)

	require.NoError(t, err)
	require.NotEqual(t, report, nil)
	for id := range report.Hits {
		require.NotEqual(t, id, "ID.20000") //verify that no invalid instructions are detected
		require.NotEqual(t, id, "ID.20007") //verify that no unknown instructions are detected
	}
}
