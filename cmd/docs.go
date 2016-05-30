package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// docsCmd represents the docs command
var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("docs called")
	},
}

func init() {
	RootCmd.AddCommand(docsCmd)
}
