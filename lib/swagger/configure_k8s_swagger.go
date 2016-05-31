package swagger

import (
	"crypto/tls"
	"net/http"

	errors "github.com/go-openapi/errors"
	runtime "github.com/go-openapi/runtime"
	middleware "github.com/go-openapi/runtime/middleware"

	"github.com/cf-furnace/k8s-stager/lib/swagger/operations"
)

func configureFlags(api *operations.K8sSwaggerAPI) {
	// api.CommandLineOptionsGroups = []swag.CommandLineOptionsGroup{ ... }
}

// ConfigureAPI configures the Stager API server
func ConfigureAPI(api *operations.K8sSwaggerAPI) http.Handler {
	// configure the api here
	api.ServeError = errors.ServeError

	api.JSONConsumer = runtime.JSONConsumer()

	api.JSONProducer = runtime.JSONProducer()

	api.StageHandler = operations.StageHandlerFunc(func(params operations.StageParams) middleware.Responder {
		return middleware.NotImplemented("operation .Stage has not yet been implemented")
	})
	api.StagingCompleteHandler = operations.StagingCompleteHandlerFunc(func(params operations.StagingCompleteParams) middleware.Responder {
		return middleware.NotImplemented("operation .StagingComplete has not yet been implemented")
	})
	api.StopStagingHandler = operations.StopStagingHandlerFunc(func(params operations.StopStagingParams) middleware.Responder {
		return middleware.NotImplemented("operation .StopStaging has not yet been implemented")
	})

	api.ServerShutdown = func() {}

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
