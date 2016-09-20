package lib

type BuildpackLifecycle struct {
	AppBitsDownloadURI             string       `json:"app_bits_download_uri,omitempty"`
	BuildArtifactsCacheDownloadURI string       `json:"build_artifacts_cache_download_uri,omitempty"`
	BuildArtifactsCacheUploadURI   string       `json:"build_artifacts_cache_upload_uri,omitempty"`
	Buildpacks                     []*Buildpack `json:"buildpacks,omitempty"`
	DropletUploadURI               string       `json:"droplet_upload_uri,omitempty"`
	Stack                          string       `json:"stack,omitempty"`
}

type Buildpack struct {
	Key  string `json:"key,omitempty"`
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

type DockerLifecycle struct {
	DockerImageUrl    string `json:"docker_image"`
	DockerLoginServer string `json:"docker_login_server,omitempty"`
	DockerUser        string `json:"docker_user,omitempty"`
	DockerPassword    string `json:"docker_password,omitempty"`
	DockerEmail       string `json:"docker_email,omitempty"`
}
