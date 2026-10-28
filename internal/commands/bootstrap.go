package commands

import (
	"context"
	"os"

	"github.com/code-gorilla-au/goety/internal/dynamodb"
	"github.com/code-gorilla-au/goety/internal/goety"
	"github.com/code-gorilla-au/goety/internal/logging"
	"github.com/code-gorilla-au/goety/internal/spinner"
	"github.com/spf13/cobra"
)

type CmdActionFunc = func(context.Context, goety.Service) error

// bootstrap instantiate common dependencies for the commands and invokes the action
func bootstrap(cmd *cobra.Command, endpoint string, action CmdActionFunc) error {
	log := logging.New(flagRootVerbose)
	ctx := cmd.Context()

	dbClient, err := dynamodb.NewClient(ctx, flagRootAwsRegion, endpoint, log)
	if err != nil {
		return err
	}

	var progress func(string)
	if !flagRootVerbose {
		spin := spinner.New(os.Stderr)
		spin.Start("starting")
		defer spin.Stop("complete")
		progress = spin.Update
	}

	g := goety.New(dbClient, log, progress, flagRootDryRun)

	if actionErr := action(ctx, g); actionErr != nil {
		return actionErr
	}

	return nil
}
