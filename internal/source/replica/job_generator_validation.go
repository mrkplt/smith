package replica

import (
	"fmt"
	"strings"
)

func validateRequest(req JobRequest) error {
	switch {
	case strings.TrimSpace(req.Namespace) == "":
		return fmt.Errorf("%w: namespace is required", ErrInvalidJobRequest)
	case len(sanitizeEtcdEndpoints(req.EtcdEndpoints)) == 0:
		return fmt.Errorf("%w: at least one etcd endpoint is required", ErrInvalidJobRequest)
	case strings.TrimSpace(req.LoopID) == "":
		return fmt.Errorf("%w: loop id is required", ErrInvalidJobRequest)
	case strings.TrimSpace(req.CorrelationID) == "":
		return fmt.Errorf("%w: correlation id is required", ErrInvalidJobRequest)
	case strings.TrimSpace(req.ServiceAccountName) == "":
		return fmt.Errorf("%w: service account is required", ErrInvalidJobRequest)
	case strings.TrimSpace(req.Image) == "":
		return fmt.Errorf("%w: image is required", ErrInvalidJobRequest)
	case strings.TrimSpace(req.Git.Repository) == "":
		return fmt.Errorf("%w: git repository is required", ErrInvalidJobRequest)
	case strings.TrimSpace(req.Git.Branch) == "":
		return fmt.Errorf("%w: git branch is required", ErrInvalidJobRequest)
	case strings.TrimSpace(req.Git.CommitSHA) == "":
		return fmt.Errorf("%w: git commit sha is required", ErrInvalidJobRequest)
	case strings.TrimSpace(req.HandoffConfigMapName) == "":
		return fmt.Errorf("%w: handoff configmap is required", ErrInvalidJobRequest)
	}
	if strings.TrimSpace(req.PRDConfigMapName) != "" {
		if strings.TrimSpace(req.PRDConfigMapKey) == "" {
			return fmt.Errorf("%w: prd configmap key is required when prd configmap is set", ErrInvalidJobRequest)
		}
		if strings.Contains(strings.TrimSpace(req.PRDConfigMapKey), "/") {
			return fmt.Errorf("%w: prd configmap key must not contain '/'", ErrInvalidJobRequest)
		}
		if workspacePathUnsafe(req.WorkspacePRDPath) {
			return fmt.Errorf("%w: workspace prd path is unsafe", ErrInvalidJobRequest)
		}
	}

	if req.BackoffLimit < 0 || req.ActiveDeadlineSeconds <= 0 || req.TTLSecondsAfterFinished < 0 {
		return fmt.Errorf("%w: invalid retry/timeout settings", ErrInvalidJobRequest)
	}
	if err := validateSkillMounts(req.SkillMounts); err != nil {
		return err
	}
	if req.RuntimeSecretName != "" && strings.TrimSpace(req.RuntimeCredentialsKey) == "" {
		return fmt.Errorf("%w: runtime credentials key is required when runtime secret is set", ErrInvalidJobRequest)
	}
	if err := validatePolicies(req); err != nil {
		return err
	}
	return validateGitAuth(req.GitAuth)
}

func validateSkillMounts(skills []SkillMount) error {
	for _, skill := range skills {
		if strings.TrimSpace(skill.Name) == "" {
			return fmt.Errorf("%w: skill mount name is required", ErrInvalidJobRequest)
		}
		if strings.TrimSpace(skill.Source) == "" {
			return fmt.Errorf("%w: skill mount source is required", ErrInvalidJobRequest)
		}
		if !strings.HasPrefix(strings.TrimSpace(skill.Source), "local://skills/") {
			return fmt.Errorf("%w: unsupported skill mount source %q", ErrInvalidJobRequest, skill.Source)
		}
		if strings.TrimSpace(skill.MountPath) == "" || !strings.HasPrefix(skill.MountPath, "/") {
			return fmt.Errorf("%w: skill mount path must be absolute", ErrInvalidJobRequest)
		}
	}
	return nil
}

func validatePolicies(req JobRequest) error {
	if req.GitPolicy != nil {
		if !req.EnableGitPolicyConfig {
			return fmt.Errorf("%w: git policy overrides are feature-flagged; set EnableGitPolicyConfig to true", ErrInvalidJobRequest)
		}
		if err := req.GitPolicy.Validate(); err != nil {
			return fmt.Errorf("%w: invalid git policy: %v", ErrInvalidJobRequest, err)
		}
	}
	if req.JournalPolicy != nil {
		if !req.EnableJournalPolicyConfig {
			return fmt.Errorf("%w: journal policy overrides are feature-flagged; set EnableJournalPolicyConfig to true", ErrInvalidJobRequest)
		}
		if err := req.JournalPolicy.Validate(); err != nil {
			return fmt.Errorf("%w: invalid journal policy: %v", ErrInvalidJobRequest, err)
		}
	}
	return nil
}

func validateGitAuth(auth *GitAuthConfig) error {
	if auth == nil {
		return nil
	}
	switch auth.Provider {
	case GitAuthProviderPAT:
		if strings.TrimSpace(auth.PATValue) == "" && (strings.TrimSpace(auth.PATSecretName) == "" || strings.TrimSpace(auth.PATSecretKey) == "") {
			return fmt.Errorf("%w: git auth provider pat requires either pat_value or pat secret name and key", ErrInvalidJobRequest)
		}
	case GitAuthProviderGitHubApp:
		if !auth.EnableGitHubAppAuth {
			return fmt.Errorf("%w: github_app auth provider is feature-flagged; set EnableGitHubAppAuth to true", ErrInvalidJobRequest)
		}
		if auth.GitHubApp == nil {
			return fmt.Errorf("%w: github_app auth provider requires github app config", ErrInvalidJobRequest)
		}
		if strings.TrimSpace(auth.GitHubApp.AppID) == "" || strings.TrimSpace(auth.GitHubApp.InstallationID) == "" {
			return fmt.Errorf("%w: github_app auth requires app id and installation id", ErrInvalidJobRequest)
		}
		if strings.TrimSpace(auth.GitHubApp.PrivateKeySecretName) == "" || strings.TrimSpace(auth.GitHubApp.PrivateKeySecretKey) == "" {
			return fmt.Errorf("%w: github_app auth requires private key secret name and key", ErrInvalidJobRequest)
		}
	case GitAuthProviderSSH:
		if !auth.EnableSSHAuth {
			return fmt.Errorf("%w: ssh auth provider is feature-flagged; set EnableSSHAuth to true", ErrInvalidJobRequest)
		}
		if auth.SSH == nil {
			return fmt.Errorf("%w: ssh auth provider requires ssh config", ErrInvalidJobRequest)
		}
		if strings.TrimSpace(auth.SSH.PrivateKeySecretName) == "" || strings.TrimSpace(auth.SSH.PrivateKeySecretKey) == "" {
			return fmt.Errorf("%w: ssh auth requires private key secret name and key", ErrInvalidJobRequest)
		}
	default:
		return fmt.Errorf("%w: unsupported git auth provider %q", ErrInvalidJobRequest, auth.Provider)
	}
	return nil
}
