package init

import (
	"github.com/mintoolkit/mint/pkg/app/master/command/containerize"
)

func init() {
	containerize.RegisterCommand()
}
