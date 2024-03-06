package execution

import (
	"github.com/mintoolkit/mint/pkg/ipc/command"
	"github.com/mintoolkit/mint/pkg/ipc/event"
)

type Interface interface {
	State() string
	Commands() <-chan command.Message
	PubEvent(etype event.Type, data ...interface{})
	Close()

	// Lifecycle hooks (extension points)
	HookSensorPostStart()
	HookSensorPreShutdown()
	HookMonitorPreStart()
	HookTargetAppRunning()
	HookMonitorPostShutdown()
	HookMonitorFailed()
}
