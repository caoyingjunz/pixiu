package types

type RepoId struct {
	Cluster string `uri:"cluster" binding:"required"`
	Id      int64  `uri:"id" binding:"required"`
}

type RepoName struct {
	Cluster string `uri:"cluster" binding:"required"`
	Name    string `uri:"name" binding:"required"`
}

type RepoURL struct {
	Url string `form:"url" binding:"required"`
}
type RepoObjectMeta struct {
	Cluster string `uri:"cluster" binding:"required"`
}

type RepoForm struct {
	Name                  string `json:"name" binding:"required"`
	URL                   string `json:"url" binding:"required"`
	Username              string `json:"username"`
	Password              string `json:"password"`
	CertFile              string `json:"certFile"`
	KeyFile               string `json:"keyFile"`
	CAFile                string `json:"caFile"`
	InsecureSkipTLSverify bool   `json:"insecure_skip_tls_verify"`
	PassCredentialsAll    bool   `json:"pass_credentials_all"`
}

type RepoUpdateForm struct {
	Name                  string `json:"name" binding:"required"`
	URL                   string `json:"url" binding:"required"`
	Username              string `json:"username"`
	Password              string `json:"password"`
	CertFile              string `json:"certFile"`
	KeyFile               string `json:"keyFile"`
	CAFile                string `json:"caFile"`
	InsecureSkipTLSverify bool   `json:"insecure_skip_tls_verify"`
	PassCredentialsAll    bool   `json:"pass_credentials_all"`
	ResourceVersion       *int64 `json:"resource_version" binding:"required"`
}

type HelmObjectMeta struct {
	Cluster   string `uri:"cluster" binding:"required"`
	Namespace string `uri:"namespace" binding:"required"`
}
