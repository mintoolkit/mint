package useragent

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"

	"github.com/depot/depot-go/internal/ci"
)

var (
	agent          string
	calculateAgent sync.Once
)

// Returns the user agent string for the CLI.
func Agent() string {
	calculateAgent.Do(func() {
		env := environmentContext{
			OS:      runtime.GOOS,
			Arch:    runtime.GOARCH,
			Version: "(devel)",
		}

		if ciProvider, isCI := ci.Provider(); isCI {
			env.IsCI = true
			env.CIProvider = ciProvider
		}

		if buildInfo, ok := debug.ReadBuildInfo(); ok {
			env.Version = buildInfo.Main.Version
		}

		asJSON, _ := json.Marshal(env)
		encoded := base64.StdEncoding.EncodeToString(asJSON)
		agent = fmt.Sprintf("depot-go/%s", encoded)
	})

	return agent
}

type environmentContext struct {
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	Version    string `json:"version"`
	IsCI       bool   `json:"isCI"`
	CIProvider string `json:"ciProvider"`
}
