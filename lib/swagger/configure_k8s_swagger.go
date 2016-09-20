package swagger

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

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

const (
	DockerLifecycleName    = "docker"
	BuildpackLifecycleName = "buildpack"
)

var (
	serverConfig *lib.ServerConfig
	Lifecycles   = map[string]string{
		DockerLifecycleName:    "",
		BuildpackLifecycleName: "",
	}
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

		if _, ok := Lifecycles[params.StagingRequest.Lifecycle]; !ok {
			serverConfig.Logger.Error(
				"Tried to stage using an unknown lifecycle.",
				fmt.Errorf("K8S stager cannot stage apps with the specified lifecycle"),
				lager.Data{
					"StagingId": params.StagingGUID,
					"Lifecycle": params.StagingRequest.Lifecycle,
				},
			)

			return &operations.StageBadRequest{}
		}

		// Staging a docker app is essentially a no-op since we're now actually
		// running the docker image. There's no reason to lookup the start
		// command or do anything ...
		if params.StagingRequest.Lifecycle == DockerLifecycleName {
			go func() {
				time.Sleep(2 * time.Second)

				dockerLifecycleData := &lib.DockerLifecycle{}
				if lifecyleDataJson, err := json.Marshal(params.StagingRequest.LifecycleData); err != nil {
					serverConfig.Logger.Error(
						"Error marshalling lifecycle data.",
						err,
						lager.Data{
							"StagingId": params.StagingGUID,
							"Org":       org,
							"Space":     space,
						},
					)
				} else {
					if err = json.Unmarshal(lifecyleDataJson, dockerLifecycleData); err != nil {
						serverConfig.Logger.Error(
							"Error marshalling lifecycle data.",
							err,
							lager.Data{
								"StagingId":     params.StagingGUID,
								"Org":           org,
								"Space":         space,
								"LifecycleData": string(lifecyleDataJson),
							},
						)
					}
				}

				ccClient := cc_client.NewCcClient(
					serverConfig.CCBaseURL,
					serverConfig.CCUsername,
					serverConfig.CCPassword,
					serverConfig.SkipCertVerification,
				)

				var annotation cc_messages.StagingTaskAnnotation

				// Based on this schema:
				// https://github.com/cloudfoundry/cloud_controller_ng/blob/173954d8ed2d2b9624d074ba2b277f7bd47c8432/lib/cloud_controller/diego/docker/staging_completion_handler.rb#L14-L24
				dockerCompletionPayload, err := json.Marshal(map[string]interface{}{
					"result": map[string]interface{}{
						"execution_metadata": "{}",
						"process_types": map[string]interface{}{
							"web": "start",
						},
						"lifecycle_type": "docker",
						"lifecycle_metadata": map[string]interface{}{
							"docker_image": dockerLifecycleData.DockerImageUrl,
						},
					},
				})

				if err != nil {
					serverConfig.Logger.Error("Error marshalling payload for CC staging complete for docker app", err)
				}

				err = ccClient.StagingComplete(
					params.StagingGUID,
					annotation.CompletionCallback,
					dockerCompletionPayload,
					serverConfig.Logger)

				if err != nil {
					serverConfig.Logger.Error("Error calling CC staging complete for docker app", err)
				}

				serverConfig.Logger.Info("Called CC staging complete for docker app")
			}()

			return &operations.StageAccepted{}
		}

		// We're now assuming that we have a buildpack lifecycle
		// since we've already dealt with the Docker one, and there's nothing else
		// for the moment

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

		buildpackLifecycleData := &lib.BuildpackLifecycle{}
		if lifecyleDataJson, err := json.Marshal(params.StagingRequest.LifecycleData); err != nil {
			serverConfig.Logger.Error(
				"Error marshalling lifecycle data.",
				err,
				lager.Data{
					"StagingId": params.StagingGUID,
					"Org":       org,
					"Space":     space,
				},
			)

			return &operations.StageInternalServerError{}
		} else {
			if err = json.Unmarshal(lifecyleDataJson, buildpackLifecycleData); err != nil {
				serverConfig.Logger.Error(
					"Error marshalling lifecycle data.",
					err,
					lager.Data{
						"StagingId":     params.StagingGUID,
						"Org":           org,
						"Space":         space,
						"LifecycleData": string(lifecyleDataJson),
					},
				)

				return &operations.StageInternalServerError{}
			}
		}

		buildpacks := make([]*k8s.Buildpack, len(buildpackLifecycleData.Buildpacks))

		for idx, buildpack := range buildpackLifecycleData.Buildpacks {
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
			Stack:            buildpackLifecycleData.Stack,
			Buildpacks:       buildpacks,
			AppLifecycleURL:  serverConfig.AppLifecycleURL,
			AppPackageURL:    buildpackLifecycleData.AppBitsDownloadURI,
			DropletUploadURL: buildpackLifecycleData.DropletUploadURI,
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
