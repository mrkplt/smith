package replica

import (
	"smith/internal/source/gitpolicy"
	"smith/internal/source/journalpolicy"
)

type GitContext struct {
	Repository string
	Branch     string
	CommitSHA  string
	UserName   string
	UserEmail  string
}

type GitAuthProvider string

const (
	GitAuthProviderPAT       GitAuthProvider = "pat"
	GitAuthProviderGitHubApp GitAuthProvider = "github_app"
	GitAuthProviderSSH       GitAuthProvider = "ssh"
)

type GitHubAppAuth struct {
	AppID                string
	InstallationID       string
	PrivateKeySecretName string
	PrivateKeySecretKey  string
}

type GitAuthConfig struct {
	Provider            GitAuthProvider
	PATSecretName       string
	PATSecretKey        string
	GitHubApp           *GitHubAppAuth
	SSH                 *SSHAuth
	EnableGitHubAppAuth bool
	EnableSSHAuth       bool
}

type SSHAuth struct {
	PrivateKeySecretName string
	PrivateKeySecretKey  string
	KnownHostsSecretName string
	KnownHostsSecretKey  string
}

type JobRequest struct {
	Namespace                     string
	EtcdEndpoints                 []string
	LoopID                        string
	CorrelationID                 string
	ProviderID                    string
	Model                         string
	InvocationMethod              string
	SourceType                    string
	SourceRef                     string
	JobName                       string
	Labels                        map[string]string
	ServiceAccountName            string
	Image                         string
	ImagePullPolicy               string
	WorkspaceSeedImage            string
	WorkspaceSeedPullPolicy       string
	Git                           GitContext
	SkillMounts                   []SkillMount
	GitPolicy                     *gitpolicy.Policy
	EnableGitPolicyConfig         bool
	JournalPolicy                 *journalpolicy.Policy
	EnableJournalPolicyConfig     bool
	GitAuth                       *GitAuthConfig
	HandoffConfigMapName          string
	PRDConfigMapName              string
	PRDConfigMapKey               string
	WorkspacePRDPath              string
	RuntimeSecretName             string
	RuntimeCredentialsKey         string
	RuntimeCredentialsValue       string
	RuntimeCredentialsClaudeKey   string
	RuntimeCredentialsClaudeValue string
	BackoffLimit                  int32
	ActiveDeadlineSeconds         int64
	TTLSecondsAfterFinished       int32
}

type JobManifest struct {
	APIVersion string
	Kind       string
	Metadata   ObjectMeta
	Spec       JobSpec
}

type ObjectMeta struct {
	Name      string
	Namespace string
	Labels    map[string]string
}

type JobSpec struct {
	BackoffLimit            int32
	ActiveDeadlineSeconds   int64
	TTLSecondsAfterFinished int32
	Template                PodTemplateSpec
}

type PodTemplateSpec struct {
	Metadata ObjectMeta
	Spec     PodSpec
}

type PodSpec struct {
	ServiceAccountName string
	RestartPolicy      string
	Volumes            []Volume
	InitContainers     []Container
	Containers         []Container
}

type Volume struct {
	Name          string
	ConfigMapName string
	Optional      bool
	EmptyDir      bool
}

type Container struct {
	Name            string
	Image           string
	ImagePullPolicy string
	Command         []string
	Env             []EnvVar
	VolumeMounts    []VolumeMount
}

type EnvVar struct {
	Name         string
	Value        string
	SecretKeyRef *SecretKeyRef
}

type SecretKeyRef struct {
	Name string
	Key  string
}

type VolumeMount struct {
	Name      string
	MountPath string
	ReadOnly  bool
}

type SkillMount struct {
	Name      string
	Source    string
	Version   string
	MountPath string
	ReadOnly  bool
}
