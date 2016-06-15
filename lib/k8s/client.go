package k8s

import (
	"fmt"

	client "github.com/kubernetes/kubernetes/pkg/client/unversioned"
	"github.com/pivotal-golang/lager"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/client/restclient"
)

type StagingInfo struct {
	DropletId       string
	DockerImageName string
	Environment     map[string]string
	Command         []string
}

type K8SStagingClient interface {
	Init(address string, logger lager.Logger) error

	CreateStagingNamespace(organization, space string) error
	GetStagingNamespace(organization, space string) error
	RemoveStagingNamespace(organization, space string) error

	StartStaging(stagingData *StagingInfo, space string) error
	GetStagingTask(stagingData *StagingInfo, space string) error
	StopStaging(stagingData *StagingInfo, space string) error
}

type Stager struct {
	URL      string
	StagerId string

	logger    lager.Logger
	k8sClient *client.Client
}

func NewStager(address string, stagerId string, logger lager.Logger) (*Stager, error) {

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
		URL:      address,
		StagerId: stagerId,

		logger:    logger,
		k8sClient: k8sClient,
	}, nil
}

func (s *Stager) CreateStagingNamespace(organization, space string) error {
	newNamespace := &api.Namespace{
		ObjectMeta: api.ObjectMeta{
			Name: formatStagingNamespace(space),
			Labels: map[string]string{
				"cf-organization": organization,
				"cf-space":        space,
				"stager-id":       s.StagerId,
			},
		},
	}

	_, err := s.k8sClient.Namespaces().Create(newNamespace)

	return err
}

func (s *Stager) GetStagingNamespace(space string) (*api.Namespace, error) {
	name := formatStagingNamespace(space)

	namespace, err := s.k8sClient.Namespaces().Get(name)

	if err != nil {
		return nil, err
	}

	return namespace, nil
}

func (s *Stager) RemoveStagingNamespace(space string) error {
	name := formatStagingNamespace(space)
	return s.k8sClient.Namespaces().Delete(name)
}

func (s *Stager) StartStaging(stagingData *StagingInfo, space string) error {
	namespace := formatStagingNamespace(space)
	name := formatStagingJobName(stagingData.DropletId)

	job := &batch.Job{
		ObjectMeta: api.ObjectMeta{
			Namespace: namespace,
			Name:      name,

			Labels: map[string]string{
				"cf-droplet-id": stagingData.DropletId,
				"cf-space":      space,
				"stager-id":     s.StagerId,
			},
		},
		Spec: batch.JobSpec{
			Template: api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Labels: map[string]string{
						"cf-droplet-id": stagingData.DropletId,
						"cf-space":      space,
						"stager-id":     s.StagerId,
					},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						api.Container{
							Name:    name,
							Image:   stagingData.DockerImageName,
							Env:     convertEnvvironmentVariables(stagingData.Environment),
							Command: stagingData.Command,
						},
					},
					RestartPolicy: api.RestartPolicyNever,
				},
			},
		},
	}

	_, err := s.k8sClient.BatchClient.Jobs(namespace).Create(job)
	return err
}

func (s *Stager) GetStagingTask(dropletId, space string) (*batch.Job, error) {
	namespace := formatStagingNamespace(space)
	name := formatStagingJobName(dropletId)

	return s.k8sClient.BatchClient.Jobs(namespace).Get(name)
}

func (s *Stager) StopStaging(dropletId, space string, gracePeriod int64) error {
	namespace := formatStagingNamespace(space)
	name := formatStagingJobName(dropletId)

	return s.k8sClient.BatchClient.Jobs(namespace).Delete(name, api.NewDeleteOptions(gracePeriod))
}

func formatStagingNamespace(space string) string {
	return fmt.Sprintf("cf-staging-%s", space)
}

func formatStagingJobName(dropletId string) string {
	return fmt.Sprintf("cf-droplet-stage-%s", dropletId)
}

func convertEnvvironmentVariables(envVars map[string]string) []api.EnvVar {
	result := []api.EnvVar{}

	for k, v := range envVars {
		envVar := api.EnvVar{
			Name:  k,
			Value: v,
		}

		result = append(result, envVar)
	}

	return result
}
