package k8s

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cf-furnace/pkg/cloudfoundry"

	"code.cloudfoundry.org/lager"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/client/restclient"
	client "k8s.io/kubernetes/pkg/client/unversioned"
)

type Buildpack struct {
	Id          string `json:"id"`
	DownloadURL string `json:"url"`
}

type StagingInfo struct {
	Id                    string
	Image                 string
	Environment           map[string]string
	Command               []string
	Stack                 string
	Buildpacks            []*Buildpack
	AppLifecycleURL       string
	AppPackageURL         string
	DropletUploadURL      string
	SkipCertVerify        bool
	SkipDetection         bool
	CompletionCallbackURL string
}

type K8SStagingClient interface {
	CreateStagingNamespace(organization, space string) error
	GetStagingNamespace(space string) (*api.Namespace, bool, error)
	RemoveStagingNamespace(space string) error

	StartStaging(stagingData *StagingInfo, space string) error
	GetStagingTask(id, space string) (*batch.Job, bool, error)
	StopStaging(id, space string, gracePeriod int64) error
}

type Stager struct {
	URL      string
	StagerId string

	logger    lager.Logger
	k8sClient *client.Client
}

func NewStager(address string, stagerId, kubeClientCertFile, kubeClientKeyFile, kubeCACertFile string, logger lager.Logger) (*Stager, error) {

	config := restclient.Config{
		Host: address,
		TLSClientConfig: restclient.TLSClientConfig{
			CertFile: kubeClientCertFile,
			KeyFile:  kubeClientKeyFile,
			CAFile:   kubeCACertFile,
		},
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

func (s *Stager) GetStagingNamespace(space string) (*api.Namespace, bool, error) {
	name := formatStagingNamespace(space)

	namespace, err := s.k8sClient.Namespaces().Get(name)

	if err != nil {
		if errors.IsNotFound(err) {
			return nil, false, nil
		} else {
			return nil, false, err
		}
	}

	return namespace, true, nil
}

func (s *Stager) RemoveStagingNamespace(space string) error {
	name := formatStagingNamespace(space)
	return s.k8sClient.Namespaces().Delete(name)
}

func (s *Stager) StartStaging(stagingData *StagingInfo, space string) error {
	namespace := formatStagingNamespace(space)

	buildpacksJSON, err := json.Marshal(stagingData.Buildpacks)
	if err != nil {
		s.logger.Error(
			"Error marshalling buildpacks JSON.",
			err,
			lager.Data{
				"StagingId":  stagingData.Id,
				"Buildpacks": stagingData.Buildpacks,
			},
		)
	}

	buildpackOrderList := make([]string, len(stagingData.Buildpacks))

	for idx, buildpack := range stagingData.Buildpacks {
		buildpackOrderList[idx] = buildpack.Id
	}

	// TODO: Write some code to either error or warn if we're overriding
	// env vars that are already set
	stagingData.Environment["CF_TASK_ID"] = stagingData.Id
	stagingData.Environment["CF_STACK"] = stagingData.Stack
	stagingData.Environment["CF_BUILDPACKS"] = string(buildpacksJSON)
	stagingData.Environment["CF_BUILDPACKS_ORDER"] = strings.Join(buildpackOrderList, ",")
	stagingData.Environment["CF_BUILDPACK_APP_LIFECYCLE"] = stagingData.AppLifecycleURL
	stagingData.Environment["CF_APP_PACKAGE"] = stagingData.AppPackageURL
	stagingData.Environment["CF_DROPLET_UPLOAD_LOCATION"] = stagingData.DropletUploadURL
	stagingData.Environment["CF_SKIP_CERT_VERIFY"] = fmt.Sprintf("%t", stagingData.SkipCertVerify)
	stagingData.Environment["CF_SKIP_DETECT"] = fmt.Sprintf("%t", stagingData.SkipDetection)
	stagingData.Environment["CF_COMPLETION_CALLBACK_URL"] = stagingData.CompletionCallbackURL

	taskGuid, err := cloudfoundry.NewTaskGuid(stagingData.Id)
	if err != nil {
		return err
	}

	vcapUid := int64(2000)

	job := &batch.Job{
		ObjectMeta: api.ObjectMeta{
			Namespace: namespace,
			Name:      taskGuid.ShortenedGuid(),
			Labels: map[string]string{
				"cloudfoundry.org/app-guid":   taskGuid.AppGuid.String(),
				"cloudfoundry.org/space-guid": space,
				"cloudfoundry.org/task-guid":  taskGuid.ShortenedGuid(),
			},
		},
		Spec: batch.JobSpec{
			Template: api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Labels: map[string]string{
						"cloudfoundry.org/app-guid":   taskGuid.AppGuid.String(),
						"cloudfoundry.org/space-guid": space,
						"cloudfoundry.org/task-guid":  taskGuid.ShortenedGuid(),
					},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						api.Container{
							Name:    "staging",
							Image:   stagingData.Image,
							Env:     convertEnvironmentVariables(stagingData.Environment),
							Command: stagingData.Command,
							SecurityContext: &api.SecurityContext{
								RunAsUser: &vcapUid,
							},
							WorkingDir: "/home/vcap/",
						},
					},
					RestartPolicy: api.RestartPolicyNever,
				},
			},
		},
	}

	_, err = s.k8sClient.BatchClient.Jobs(namespace).Create(job)
	return err
}

func (s *Stager) GetStagingTask(id, space string) (*batch.Job, bool, error) {
	namespace := formatStagingNamespace(space)
	taskGuid, err := cloudfoundry.NewTaskGuid(id)
	if err != nil {
		return nil, false, err
	}

	result, err := s.k8sClient.BatchClient.Jobs(namespace).Get(taskGuid.ShortenedGuid())

	if err != nil {
		if errors.IsNotFound(err) {
			return nil, false, nil
		} else {
			return nil, false, err
		}
	}

	return result, true, nil
}

func (s *Stager) StopStaging(id, space string, gracePeriod int64) error {
	namespace := formatStagingNamespace(space)
	taskGuid, err := cloudfoundry.NewTaskGuid(id)
	if err != nil {
		return err
	}

	return s.k8sClient.BatchClient.Jobs(namespace).Delete(taskGuid.ShortenedGuid(), nil)
}

func formatStagingNamespace(space string) string {
	return fmt.Sprintf("cf-staging-%s", space)
}

func convertEnvironmentVariables(envVars map[string]string) []api.EnvVar {
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
