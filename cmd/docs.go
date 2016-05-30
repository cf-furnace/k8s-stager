package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// docsCmd represents the docs command
var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Subcommands for generating documentation.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("docs called")
	},
}

func init() {
	RootCmd.AddCommand(docsCmd)
}
