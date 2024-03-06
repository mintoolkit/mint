package init

import (
	"github.com/mintoolkit/mint/pkg/app/master/command/version"
)

func init() {
	version.RegisterCommand()
}
