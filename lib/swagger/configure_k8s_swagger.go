package swagger

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"

	"github.com/cf-furnace/k8s-stager/lib"
	"github.com/cf-furnace/k8s-stager/lib/k8s"
	"github.com/cf-furnace/k8s-stager/lib/swagger/operations"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/stager/cc_client"
	errors "github.com/go-openapi/errors"
	runtime "github.com/go-openapi/runtime"
	middleware "github.com/go-openapi/runtime/middleware"
)

var (
	serverConfig *lib.ServerConfig
)

// ConfigureAPI configures the Stager API server
func ConfigureAPI(api *operations.K8sSwaggerAPI, serverConfiguration *lib.ServerConfig) http.Handler {
	serverConfig = serverConfiguration

	// configure the api here
	api.ServeError = errors.ServeError
	api.JSONConsumer = runtime.JSONConsumer()
	api.JSONProducer = runtime.JSONProducer()

	// Since we don't have the information from the CC yet, use these values
	// in lieu of an actual org and space
	org := "cf-furnace"
	space := serverConfig.K8SNamespace

	api.StageHandler = operations.StageHandlerFunc(func(params operations.StageParams) middleware.Responder {
		serverConfig.Logger.Debug("Stage called", lager.Data{
			"StagingGuid":    params.StagingGUID,
			"StagingRequest": params.StagingRequest,
		})

		if params.StagingRequest.Lifecycle != "buildpack" {
			serverConfig.Logger.Error(
				"Tried to stage using a lifecycle other than 'buildpack'",
				fmt.Errorf("K8S stager can only stage apps wilth a 'buildpack' lifecycle"),
				lager.Data{
					"StagingId": params.StagingGUID,
					"Lifecycle": params.StagingRequest.Lifecycle,
				},
			)

			return &operations.StageBadRequest{}
		}

		_, namespaceExists, err := serverConfig.K8SClient.GetStagingNamespace(space)

		if err != nil {
			serverConfig.Logger.Error(
				"Error looking up k8s namespace.",
				err,
				lager.Data{
					"StagingId": params.StagingGUID,
					"Org":       org,
					"Space":     space,
				},
			)

			return &operations.StageInternalServerError{}
		}

		if !namespaceExists {
			serverConfig.Logger.Info(
				"Staging namespace does not exist. Creating it.",
				lager.Data{
					"StagingId": params.StagingGUID,
				},
			)

			err = serverConfig.K8SClient.CreateStagingNamespace(org, space)

			if err != nil {
				serverConfig.Logger.Error(
					"Error creating k8s namespace.",
					err,
					lager.Data{
						"StagingId": params.StagingGUID,
						"Org":       org,
						"Space":     space,
					},
				)

				return &operations.StageInternalServerError{}
			}
		} else {
			serverConfig.Logger.Debug(
				"Staging namespace already exists. Not creating again.",
				lager.Data{
					"StagingId": params.StagingGUID,
				},
			)
		}

		env := map[string]string{}
		for _, envEntry := range params.StagingRequest.Environment {
			env[envEntry.Name] = envEntry.Value
		}

		buildpacks := make([]*k8s.Buildpack, len(params.StagingRequest.LifecycleData.Buildpacks))

		for idx, buildpack := range params.StagingRequest.LifecycleData.Buildpacks {
			buildpacks[idx] = &k8s.Buildpack{
				Id:          buildpack.Key,
				DownloadURL: buildpack.URL,
			}
		}

		var command []string

		if len(serverConfig.CustomImageCommand) == 0 {
			command = nil
		} else {
			command = strings.Split(serverConfig.CustomImageCommand, " ")
		}

		stagingInfo := &k8s.StagingInfo{
			Id:               params.StagingGUID,
			Image:            serverConfig.StagingImage,
			Environment:      env,
			Command:          command,
			Stack:            params.StagingRequest.LifecycleData.Stack,
			Buildpacks:       buildpacks,
			AppLifecycleURL:  serverConfig.AppLifecycleURL,
			AppPackageURL:    params.StagingRequest.LifecycleData.AppBitsDownloadURI,
			DropletUploadURL: params.StagingRequest.LifecycleData.DropletUploadURI,
			SkipCertVerify:   serverConfig.SkipCertVerification,
			SkipDetection:    false,
			CompletionCallbackURL: fmt.Sprintf(
				"http://%s:%d/v1/staging/%s/completed",
				serverConfig.AdvertiseAddress,
				serverConfig.Port,
				params.StagingGUID,
			),
		}

		serverConfig.Logger.Info(
			"Trying to run staging job.",
			lager.Data{
				"StagingId": params.StagingGUID,
			},
		)

		err = serverConfig.K8SClient.StartStaging(stagingInfo, space)

		if err != nil {
			serverConfig.Logger.Error(
				"Error running staging job.",
				err,
				lager.Data{
					"StagingId": params.StagingGUID,
					"Org":       org,
					"Space":     space,
				},
			)

			return &operations.StageInternalServerError{}
		}

		return &operations.StageAccepted{}
	})

	api.StagingCompleteHandler = operations.StagingCompleteHandlerFunc(func(params operations.StagingCompleteParams) middleware.Responder {
		serverConfig.Logger.Debug("Stage complete called", lager.Data{
			"StagingGuid":            params.StagingGUID,
			"StagingCompleteRequest": params.StagingCompleteRequest,
		})

		ccClient := cc_client.NewCcClient(
			serverConfig.CCBaseURL,
			serverConfig.CCUsername,
			serverConfig.CCPassword,
			serverConfig.SkipCertVerification,
		)

		var annotation cc_messages.StagingTaskAnnotation

		err := ccClient.StagingComplete(
			params.StagingCompleteRequest.TaskGUID,
			annotation.CompletionCallback,
			[]byte(params.StagingCompleteRequest.Result),
			serverConfig.Logger)

		if err != nil {
			serverConfig.Logger.Error("Error calling CC staging complete", err)
			if _, ok := err.(*cc_client.BadResponseError); ok {
				return &operations.StagingCompleteBadRequest{}
			} else {
				return &operations.StagingCompleteServiceUnavailable{}
			}

			return &operations.StagingCompleteNotFound{}
		}

		serverConfig.Logger.Info("Called CC staging complete")

		serverConfig.Logger.Info("Removing staging job")

		// Delete the job from Kubernetes
		err = serverConfig.K8SClient.StopStaging(params.StagingGUID, params.StagingCompleteRequest.Space, serverConfig.StagingStopGracePeriodSeconds)

		if err != nil {
			serverConfig.Logger.Error(
				"Error deleting the staging job.",
				err,
				lager.Data{
					"StagingId": params.StagingGUID,
				},
			)

			return &operations.StagingCompleteServiceUnavailable{}
		}

		return &operations.StagingCompleteOK{}
	})

	api.StopStagingHandler = operations.StopStagingHandlerFunc(func(params operations.StopStagingParams) middleware.Responder {
		serverConfig.Logger.Debug("Stage stop called", lager.Data{
			"StagingGuid": params.StagingGUID,
		})

		_, exists, err := serverConfig.K8SClient.GetStagingTask(params.StagingGUID, space)
		if err != nil {
			serverConfig.Logger.Error(
				"Error looking up staging task to stop.",
				err,
				lager.Data{
					"StagingId": params.StagingGUID,
				},
			)

			return &operations.StopStagingInternalServerError{}
		}

		if exists {

			serverConfig.Logger.Info(
				"Trying to stop staging job.",
				lager.Data{
					"StagingId": params.StagingGUID,
				},
			)

			err = serverConfig.K8SClient.StopStaging(
				params.StagingGUID,
				space,
				serverConfig.StagingStopGracePeriodSeconds,
			)

			if err != nil {
				serverConfig.Logger.Error(
					"Error stopping staging task.",
					err,
					lager.Data{
						"StagingId": params.StagingGUID,
					},
				)

				return &operations.StopStagingInternalServerError{}
			}

			return &operations.StopStagingAccepted{}
		}

		return &operations.StopStagingNotFound{}
	})

	api.ServerShutdown = func() {
		serverConfig.Logger.Info("Server is shutting down.", lager.Data{})
	}

	return setupGlobalMiddleware(api.Serve(setupMiddlewares))
}

// The TLS configuration before HTTPS server starts.
func configureTLS(tlsConfig *tls.Config) {
	// Make all necessary changes to the TLS configuration here.
}

// The middleware configuration is for the handler executors. These do not apply to the swagger.json document.
// The middleware executes after routing but before authentication, binding and validation
func setupMiddlewares(handler http.Handler) http.Handler {
	return handler
}

// The middleware configuration happens before anything, this middleware also applies to serving the swagger.json document.
// So this is a good place to plug in a panic handling middleware, logging and metrics
func setupGlobalMiddleware(handler http.Handler) http.Handler {
	return handler
}
