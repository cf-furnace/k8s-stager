package k8s

import (
	"fmt"

	client "github.com/kubernetes/kubernetes/pkg/client/unversioned"
	"github.com/pivotal-golang/lager"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/restclient"
)

type StagingInfo struct {
	DropletId                   string
	PackageDownloadURL          string
	LifecycleBundleDownloadURL  string
	DockerImageName             string
	DropletUploadDestinationURL string
}

type K8SStagingClient interface {
	Init(address string, logger lager.Logger) error

	CreateStagingNamespace(name string) error
	GetStagingNamespace(name string) error
	RemoveStagingNamespace(name string) error

	StartStaging(stagingData *StagingInfo) error
	GetStagingTask(stagingData *StagingInfo) error
	StopStaging(stagingData *StagingInfo) error
}

type Stager struct {
	URL       string
	logger    lager.Logger
	k8sClient *client.Client
}

func NewStager(address string, logger lager.Logger) (*Stager, error) {

	config := restclient.Config{
		Host: address,
	}

	logger.Info("Trying to connect to Kubernetes API", lager.Data{"k8s_api_url": address})

	k8sClient, err := client.New(&config)
	if err != nil {
		logger.Error("Can't create Kubernetes Client", err, lager.Data{"address": address})
		return nil, fmt.Errorf("Can't create Kubernetes client; k8s api address: %s", address)
	}

	_, err = k8sClient.ServerVersion()
	if err != nil {
		logger.Error("Can't connect to Kubernetes API", err, lager.Data{"address": address})
		return nil, fmt.Errorf("Can't connect to Kubernetes API %s: %V", address, err)
	}

	logger.Debug("Connected to Kubernetes API %s", lager.Data{"address": address})

	return &Stager{
		URL:       address,
		logger:    logger,
		k8sClient: k8sClient,
	}, nil
}

func (s *Stager) CreateStagingNamespace(organization, space string) error {

	//	newNamespace = &api.Namespace{
	//		Name: fmt.Sprintf("cf-staging-%s-%s", organization, space),
	//		Labels: map[string]string{
	//			"cf-organization": organization,
	//			"cf-space":        space,
	//		},
	//	}

	//	s.k8sClient.Namespaces().Create(newNamespace)

	return nil
}

func (s *Stager) GetStagingNamespace(name string) (*api.Namespace, error) {
	namespace, err := s.k8sClient.Namespaces().Get(name)

	if err != nil {
		return nil, err
	}

	return namespace, nil
}

func (s *Stager) RemoveStagingNamespace(name string) error {
	return nil
}

func (s *Stager) StartStaging(stagingData *StagingInfo) error {
	return nil
}

func (s *Stager) GetStagingTask(stagingData *StagingInfo) error {
	return nil
}

func (s *Stager) StopStaging(stagingData *StagingInfo) error {
	return nil
}
