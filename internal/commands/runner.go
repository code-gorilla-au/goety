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

func bootstrap(cmd *cobra.Command, endpoint string, action CmdActionFunc) error {
	log := logging.New(flagRootVerbose)
	ctx := context.Background()

	dbClient, err := dynamodb.NewClient(ctx, flagRootAwsRegion, endpoint)
	if err != nil {
		return err
	}

	var progress func(string)
	if !flagRootVerbose {
		spin := spinner.New(os.Stderr)
		spin.Start("starting dump")
		defer spin.Stop("dump complete")
		progress = spin.Update
	}

	g := goety.New(dbClient, log, progress, flagRootDryRun)

	if actionErr := action(ctx, g); actionErr != nil {
		log.Error("error running action", "error", actionErr)
		return actionErr
	}

	return nil
}
