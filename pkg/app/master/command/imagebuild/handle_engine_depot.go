package imagebuild

import (
	"context"
	"time"

	"github.com/depot/depot-go/build"
	"github.com/depot/depot-go/machine"
	cliv1 "github.com/depot/depot-go/proto/depot/cli/v1"
	"github.com/moby/buildkit/client"
	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
)

// HandleDepotEngine implements support for the Depot.dev container build engine
func HandleDepotEngine(
	logger *log.Entry,
	xc *app.ExecutionContext,
	gparams *command.GenericParams,
	cparams *CommandParams) {
	logger.Trace("HandleDepotEngine.call")
	defer logger.Trace("HandleDepotEngine.exit")
	ctx := context.Background()

	var doLoad bool
	if cparams.Runtime != NoneRuntimeLoad {
		doLoad = true
	}

	req := &cliv1.CreateBuildRequest{
		ProjectId: cparams.EngineNamespace,
		Options: []*cliv1.BuildOptions{
			{
				Command: cliv1.Command_COMMAND_BUILD,
				Tags:    []string{cparams.ImageName},
				Load:    doLoad,
				Outputs: []*cliv1.BuildOutput{
					{
						Kind: "docker",
						Attributes: map[string]string{
							"name": cparams.ImageName,
							"dest": cparams.ImageArchiveFile,
						},
					},
				},
			},
		},
	}

	logger.Trace("depot.build.NewBuild")
	build, err := build.NewBuild(ctx, req, cparams.EngineToken)
	if err != nil {
		panic(err)
	}
	logger.Tracef("depot.build.NewBuild -> id=%s buildURL='%s' useLocalRegistry=%v proxyImage=%s",
		build.ID, build.BuildURL, build.UseLocalRegistry, build.ProxyImage)

	var berr error
	defer func() {
		build.Finish(berr)
		xc.FailOn(berr)
	}()

	logger.Tracef("depot.machine.Acquire(%s) - \n", build.ID)
	var bmachine *machine.Machine
	bmachine, berr = machine.Acquire(ctx, build.ID, build.Token, cparams.Architecture)
	if berr != nil {
		return
	}
	defer bmachine.Release()

	connectCtx, cancelConnect := context.WithTimeout(ctx, 5*time.Minute)
	defer cancelConnect()

	logger.Trace("depot.bmachine.Connect")
	var bclient *client.Client
	bclient, berr = bmachine.Connect(connectCtx)
	if berr != nil {
		return
	}

	berr = buildkitBuildImage(logger, xc, cparams, ctx, bclient)
	if berr != nil {
		return
	}
}
