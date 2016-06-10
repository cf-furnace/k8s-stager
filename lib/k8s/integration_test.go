// +build integration
package k8s

import (
	"os"
	"testing"

	"github.com/cf-furnace/k8s-stager/lib"

	"github.com/pivotal-golang/lager"
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
	_, err := NewStager(k8sApiUrl, logger)

	// Assert
	assert.NoError(err)
}

func TestConnectionNotOK(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	badAddress := "127.0.0.1:1"

	// Act
	_, err := NewStager(badAddress, logger)

	// Assert
	assert.Error(err)
}
