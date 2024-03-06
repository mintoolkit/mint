package init

import (
	"github.com/mintoolkit/mint/pkg/app/master/command/lint"
)

func init() {
	lint.RegisterCommand()
}
