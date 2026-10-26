package commands

import "github.com/spf13/cobra"

var (
	flagRootVerbose   = false
	flagRootDryRun    = false
	flagRootAwsRegion = "ap-southeast-2"
)

var rootCmd = &cobra.Command{
	Use:          "goety [COMMAND] --[FLAGS]",
	Short:        "dynamodb power tools",
	Long:         "Power tools to interact with dynamodb tables",
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&flagRootVerbose, "verbose", "v", false, "add verbose logging")
	rootCmd.PersistentFlags().BoolVarP(&flagRootDryRun, "dry-run", "d", false, "dry run does not perform actions, only logs them")
	rootCmd.PersistentFlags().StringVarP(&flagRootAwsRegion, "aws-region", "r", "ap-southeast-2", "aws region the table is located")

	rootCmd.AddCommand(purgeCmd)
	rootCmd.AddCommand(dumpCmd)
	rootCmd.AddCommand(seedCmd)
}

func Execute() error {

	return rootCmd.Execute()
}
