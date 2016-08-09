package lib

import (
	"code.cloudfoundry.org/lager"
	"github.com/cf-furnace/k8s-stager/lib/k8s"
)

type ServerConfig struct {
	LogLevel                      string
	Listen                        string
	Port                          int
	AdvertiseAddress              string
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
	KBSClientCertFile             string
	K8SClientKeyFile              string
	K8SCACertFile                 string
	CCBaseURL                     string
	CCUsername                    string
	CCPassword                    string
}
