package lib

import (
	"github.com/cf-furnace/k8s-stager/lib/k8s"
	"github.com/pivotal-golang/lager"
)

type ServerConfig struct {
	LogLevel                      string
	Listen                        string
	StagerId                      string
	StagingImage                  string
	K8SAPIEndpoint                string
	StagingStopGracePeriodSeconds int64
	K8SNamespace                  string
	Logger                        lager.Logger
	K8SClient                     k8s.K8SStagingClient
	SkipCertVerification          bool
	AppLifecycleURL               string
	CustomImageCommand            string
}
