package cmd

import (
	"net/http"

	"github.com/cf-furnace/k8s-stager/lib"
	"github.com/cf-furnace/k8s-stager/lib/swagger"
	"github.com/cf-furnace/k8s-stager/lib/swagger/operations"

	spec "github.com/go-swagger/go-swagger/spec"
	"github.com/pivotal-golang/lager"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagLogLevel string
	flagListen   string
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs the Kubernetes Cloud Foundry Stager",
	Run: func(cmd *cobra.Command, args []string) {

		// Load configuration
		flagLogLevel = viper.GetString("log-level")
		flagListen = viper.GetString("listen")

		// Create a logger
		logger := lib.NewLogger(flagLogLevel)

		// Load swagger spec
		swaggerSpec, err := spec.New(swagger.SwaggerJSON, "")
		if err != nil {
			logger.Fatal("initializing-swagger-failed", err)
		}

		logger.Info("start-listening", lager.Data{"address": flagListen})

		api := operations.NewK8sSwaggerAPI(swaggerSpec)

		stagerServer := swagger.ConfigureAPI(api)

		err = http.ListenAndServe(flagListen, stagerServer)
		if err != nil {
			logger.Fatal("listening-failed", err)
		}
	},
}

func init() {
	RootCmd.AddCommand(runCmd)

	runCmd.PersistentFlags().StringP(
		"listen",
		"l",
		"0.0.0.0:8080",
		"Address to listen on.",
	)

	runCmd.PersistentFlags().StringP(
		"log-level",
		"L",
		"info",
		"Logging level.",
	)

	viper.BindPFlags(runCmd.PersistentFlags())
}
