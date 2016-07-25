// +build integration
package k8s

import (
	"os"
	"testing"
	"time"

	"github.com/cf-furnace/k8s-stager/lib/logger"

	"github.com/pivotal-golang/lager"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
)

const (
	k8sApiUrlEnvVar = "CF_STAGER_INTEGRATION_K8S_ADDRESS"
)

var k8sApiUrl string
var logger lager.Logger

func TestMain(m *testing.M) {
	logger = lib.NewLogger("debug")

	k8sApiUrl = os.Getenv(k8sApiUrlEnvVar)
	if k8sApiUrl == "" {
		logger.Fatal("Please set CF_STAGER_INTEGRATION_K8S_ADDRESS before running integration tests.", nil, lager.Data{})
	}

	retCode := m.Run()

	os.Exit(retCode)
}

func TestConnectionOK(t *testing.T) {
	// Arrange
	assert := assert.New(t)

	// Act
	_, err := NewStager(k8sApiUrl, "foo", logger)

	// Assert
	assert.NoError(err)
}

func TestConnectionNotOK(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	badAddress := "127.0.0.1:1"

	// Act
	_, err := NewStager(badAddress, "foo", logger)

	// Assert
	assert.Error(err)
}

func TestGetNamespaceNotOK(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	badAddress := k8sApiUrl
	stager, err := NewStager(badAddress, "foo", logger)
	assert.NoError(err)

	// Act
	_, exists, err := stager.GetStagingNamespace("foo")

	// Assert
	assert.NoError(err)
	assert.False(exists)
}

func TestCreateNamespaceOK(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	badAddress := k8sApiUrl
	stager, err := NewStager(badAddress, "foo", logger)
	assert.NoError(err)

	org := uuid.NewV4().String()
	space := uuid.NewV4().String()

	// Act
	err = stager.CreateStagingNamespace(org, space)
	namespace, err2 := stager.GetStagingNamespace(space)

	// Assert
	assert.NoError(err)
	assert.NoError(err2)
	assert.Equal(formatStagingNamespace(space), namespace.Name)

	// Cleanup
	err = stager.RemoveStagingNamespace(space)
	assert.NoError(err)
}

func TestStagingTaskOk(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	badAddress := k8sApiUrl
	stager, err := NewStager(badAddress, "foo", logger)
	assert.NoError(err)
	org := uuid.NewV4().String()
	space := uuid.NewV4().String()
	err = stager.CreateStagingNamespace(org, space)
	assert.NoError(err)

	dropletId := uuid.NewV4().String()
	message := uuid.NewV4().String()

	stagingData := &StagingInfo{
		DropletId:       dropletId,
		DockerImageName: "alpine:latest",
		Environment: map[string]string{
			"MESSAGE": message,
		},
		Command: []string{"sh", "-c", "echo $MESSAGE"},
	}

	// Act
	err = stager.StartStaging(stagingData, space)
	assert.NoError(err)

	job, err := stager.GetStagingTask(dropletId, space)

	for ; err == nil && job.Status.Failed == 0 && job.Status.Succeeded == 0; job, err = stager.GetStagingTask(dropletId, space) {
		time.Sleep(1 * time.Second)
	}

	// Assert
	assert.NoError(err)
	assert.Equal(1, job.Status.Succeeded)
	assert.Equal(0, job.Status.Failed)

	// Cleanup
	err = stager.RemoveStagingNamespace(space)
	assert.NoError(err)
}
