package init

import (
	"github.com/mintoolkit/mint/pkg/app/master/command/vulnerability"
)

func init() {
	vulnerability.RegisterCommand()
}
