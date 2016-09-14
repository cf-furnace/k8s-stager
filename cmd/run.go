package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/cf-furnace/k8s-stager/lib"
	"github.com/cf-furnace/k8s-stager/lib/k8s"
	"github.com/cf-furnace/k8s-stager/lib/logger"
	"github.com/cf-furnace/k8s-stager/lib/swagger"
	"github.com/cf-furnace/k8s-stager/lib/swagger/operations"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry-incubator/locket"
	"github.com/go-openapi/loads"
	"github.com/hashicorp/consul/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs the Kubernetes Cloud Foundry Stager",
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		serverConfig := &lib.ServerConfig{}

		// Load configuration
		serverConfig.LogLevel = viper.GetString("log-level")
		serverConfig.Listen = viper.GetString("listen")
		serverConfig.Port = viper.GetInt("port")
		serverConfig.AdvertiseAddress = viper.GetString("advertise-address")
		serverConfig.StagerId = viper.GetString("id")
		serverConfig.StagingImage = viper.GetString("staging-image")
		serverConfig.K8SAPIEndpoint = viper.GetString("k8s-endpoint")
		serverConfig.K8SNamespace = viper.GetString("k8s-namespace")
		serverConfig.StagingStopGracePeriodSeconds = int64(viper.GetDuration("k8s-endpoint").Seconds())
		serverConfig.SkipCertVerification = viper.GetBool("skip-cert-verify")
		serverConfig.AppLifecycleURL = viper.GetString("app-lifecycle-url")
		serverConfig.CustomImageCommand = viper.GetString("custom-image-command")
		serverConfig.KBSClientCertFile = viper.GetString("k8s-client-cert")
		serverConfig.K8SClientKeyFile = viper.GetString("k8s-client-key")
		serverConfig.K8SCACertFile = viper.GetString("k8s-cacert")
		serverConfig.CCBaseURL = viper.GetString("cc-baseurl")
		serverConfig.CCUsername = viper.GetString("cc-username")
		serverConfig.CCPassword = viper.GetString("cc-password")
		serverConfig.ConsulCluster = viper.GetString("consul-cluster")

		// Create a logger
		serverConfig.Logger = logger.NewLogger(serverConfig.LogLevel)

		// Connect to Kubernetes
		serverConfig.K8SClient, err = k8s.NewStager(
			serverConfig.K8SAPIEndpoint,
			serverConfig.StagerId,
			serverConfig.KBSClientCertFile,
			serverConfig.K8SClientKeyFile,
			serverConfig.K8SCACertFile,
			serverConfig.Logger,
		)

		if err != nil {
			serverConfig.Logger.Fatal(
				"Could not connect to Kubernetes",
				err,
				lager.Data{
					"K8SEndpoint": serverConfig.K8SAPIEndpoint,
				},
			)
		}

		// Load swagger spec
		swaggerSpec, err := loads.Analyzed(swagger.SwaggerJSON, "")
		if err != nil {
			serverConfig.Logger.Fatal("initializing-swagger-failed", err)
		}

		serverConfig.Logger.Info(
			"start-listening",
			lager.Data{
				"address": serverConfig.Listen,
				"port":    serverConfig.Port,
			},
		)

		api := operations.NewK8sSwaggerAPI(swaggerSpec)

		stagerServer := swagger.ConfigureAPI(api, serverConfig)

		consulClient, err := consuladapter.NewClientFromUrl(serverConfig.ConsulCluster)
		if err != nil {
			serverConfig.Logger.Fatal("new-consul-client-failed", err)
		}

		clock := clock.NewClock()
		registrationRunner := initializeRegistrationRunner(serverConfig.Logger, consulClient, serverConfig.Port, clock)

		members := grouper.Members{
			{"server", http_server.New(fmt.Sprintf("%s:%d", serverConfig.Listen, serverConfig.Port), stagerServer)},
			{"registration-runner", registrationRunner},
		}

		group := grouper.NewOrdered(os.Interrupt, members)

		monitor := ifrit.Invoke(sigmon.New(group))

		serverConfig.Logger.Info("started")

		err = <-monitor.Wait()
		if err != nil {
			serverConfig.Logger.Error("exited-with-failure", err)
			os.Exit(1)
		}

		serverConfig.Logger.Info("exited")
	},
}

func initializeRegistrationRunner(
	logger lager.Logger,
	consulClient consuladapter.Client,
	port int,
	clock clock.Clock) ifrit.Runner {
	registration := &api.AgentServiceRegistration{
		Name: "stager",
		Port: port,
		Check: &api.AgentServiceCheck{
			TTL: "3s",
		},
	}
	return locket.NewRegistrationRunner(logger, registration, consulClient, locket.RetryInterval, clock)
}

func init() {
	RootCmd.AddCommand(runCmd)

	runCmd.PersistentFlags().StringP(
		"listen",
		"l",
		"0.0.0.0",
		"Address to listen on.",
	)

	runCmd.PersistentFlags().IntP(
		"port",
		"",
		8080,
		"Port to listen on.",
	)

	runCmd.PersistentFlags().StringP(
		"advertise-address",
		"",
		"",
		"Address of stager, as used by other components.",
	)

	runCmd.PersistentFlags().StringP(
		"log-level",
		"L",
		"info",
		"Logging level.",
	)

	runCmd.PersistentFlags().StringP(
		"id",
		"i",
		"stager-0",
		"Identifier of the stager.",
	)

	runCmd.PersistentFlags().StringP(
		"staging-image",
		"s",
		"cffurnace/stager",
		"Image to use for staging.",
	)

	runCmd.PersistentFlags().StringP(
		"k8s-endpoint",
		"k",
		"",
		"Kubernetes HTTP API endpoint.",
	)

	runCmd.PersistentFlags().StringP(
		"k8s-namespace",
		"n",
		"furnace-staging",
		"Kubernetes namespace to use for staging.",
	)

	runCmd.PersistentFlags().DurationP(
		"stage-stop-grace",
		"g",
		time.Second*60,
		"Grace period for staging stop.",
	)

	runCmd.PersistentFlags().BoolP(
		"skip-cert-verify",
		"S",
		false,
		"Skip certificate validation when staging.",
	)

	runCmd.PersistentFlags().StringP(
		"app-lifecycle-url",
		"a",
		"",
		"Application lifecycle URL.",
	)

	runCmd.PersistentFlags().StringP(
		"custom-image-command",
		"c",
		"",
		"Custom entrypoint to use when running the staging command.",
	)

	runCmd.PersistentFlags().StringP(
		"k8s-client-cert",
		"",
		"",
		"Path to a PEM-encoded client certificate.",
	)

	runCmd.PersistentFlags().StringP(
		"k8s-client-key",
		"",
		"",
		"Path to a PEM-encoded client key.",
	)

	runCmd.PersistentFlags().StringP(
		"k8s-cacert",
		"",
		"",
		"Path to a PEM-encoded CA certificate for connecting to kubernetes.",
	)

	runCmd.PersistentFlags().StringP(
		"cc-baseurl",
		"",
		"",
		"Cloud controller API location.",
	)

	runCmd.PersistentFlags().StringP(
		"cc-username",
		"",
		"",
		"Cloud Controller internal API username.",
	)

	runCmd.PersistentFlags().StringP(
		"cc-password",
		"",
		"",
		"Cloud Controller internal API password.",
	)

	runCmd.PersistentFlags().StringP(
		"consul-cluster",
		"",
		"",
		"Consul used for service discovery.",
	)

	viper.BindPFlags(runCmd.PersistentFlags())
}
