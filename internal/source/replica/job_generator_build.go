package replica

import (
	"fmt"
	"path"
	"strings"
)

func BuildReplicaJob(req JobRequest) (JobManifest, error) {
	req = applyPRDDefaults(req)
	if err := validateRequest(req); err != nil {
		return JobManifest{}, err
	}
	if req.ImagePullPolicy == "" {
		req.ImagePullPolicy = "IfNotPresent"
	}

	jobName := resolveJobName(req)
	labels := buildReplicaLabels(req)
	env := buildReplicaEnv(req)
	volumes, volumeMounts, hasPRDConfig := buildReplicaVolumes(req)
	initContainers := buildReplicaInitContainers(req, hasPRDConfig)

	return JobManifest{
		APIVersion: "batch/v1",
		Kind:       "Job",
		Metadata: ObjectMeta{
			Name:      jobName,
			Namespace: req.Namespace,
			Labels:    labels,
		},
		Spec: JobSpec{
			BackoffLimit:            req.BackoffLimit,
			ActiveDeadlineSeconds:   req.ActiveDeadlineSeconds,
			TTLSecondsAfterFinished: req.TTLSecondsAfterFinished,
			Template: PodTemplateSpec{
				Metadata: ObjectMeta{
					Labels: labels,
				},
				Spec: PodSpec{
					ServiceAccountName: req.ServiceAccountName,
					RestartPolicy:      "Never",
					Volumes:            volumes,
					InitContainers:     initContainers,
					Containers: []Container{
						{
							Name:            "replica",
							Image:           req.Image,
							ImagePullPolicy: req.ImagePullPolicy,
							Command:         []string{"/bin/smith", "replica", "run"},
							Env:             env,
							VolumeMounts:    volumeMounts,
						},
					},
				},
			},
		},
	}, nil
}

func applyPRDDefaults(req JobRequest) JobRequest {
	if strings.TrimSpace(req.PRDConfigMapName) == "" {
		return req
	}
	if strings.TrimSpace(req.PRDConfigMapKey) == "" {
		req.PRDConfigMapKey = "prd.json"
	}
	if strings.TrimSpace(req.WorkspacePRDPath) == "" {
		req.WorkspacePRDPath = ".agents/tasks/prd.json"
	}
	return req
}

func resolveJobName(req JobRequest) string {
	jobName := strings.TrimSpace(req.JobName)
	if jobName != "" {
		return jobName
	}
	return fmt.Sprintf("smith-replica-%s", sanitizeName(req.LoopID))
}

func buildReplicaLabels(req JobRequest) map[string]string {
	labels := map[string]string{
		"app.kubernetes.io/name":      "smith-replica",
		"app.kubernetes.io/component": "replica",
		"smith.io/loop-id":            sanitizeKubernetesLabelValue(req.LoopID),
		"smith.io/correlation-id":     sanitizeKubernetesLabelValue(req.CorrelationID),
	}
	for key, raw := range req.Labels {
		k := strings.TrimSpace(key)
		if k == "" {
			continue
		}
		labels[k] = sanitizeKubernetesLabelValue(raw)
	}
	return labels
}

func buildReplicaEnv(req JobRequest) []EnvVar {
	env := []EnvVar{
		{Name: "STORY_ID", Value: req.LoopID},
		{Name: "SMITH_LOOP_ID", Value: req.LoopID},
		{Name: "SMITH_CORRELATION_ID", Value: req.CorrelationID},
		{Name: "SMITH_ETCD_ENDPOINTS", Value: strings.Join(sanitizeEtcdEndpoints(req.EtcdEndpoints), ",")},
		{Name: "SMITH_LOOP_PROVIDER", Value: strings.ToLower(strings.TrimSpace(req.ProviderID))},
		{Name: "SMITH_LOOP_MODEL", Value: strings.TrimSpace(req.Model)},
		{Name: "SMITH_LOOP_INVOCATION_METHOD", Value: strings.TrimSpace(req.InvocationMethod)},
		{Name: "SMITH_LOOP_SOURCE_TYPE", Value: strings.TrimSpace(req.SourceType)},
		{Name: "SMITH_LOOP_SOURCE_REF", Value: strings.TrimSpace(req.SourceRef)},
		{Name: "SMITH_GIT_REPOSITORY", Value: req.Git.Repository},
		{Name: "SMITH_GIT_BRANCH", Value: req.Git.Branch},
		{Name: "SMITH_GIT_COMMIT_SHA", Value: req.Git.CommitSHA},
		{Name: "SMITH_GIT_USER_NAME", Value: req.Git.UserName},
		{Name: "SMITH_GIT_USER_EMAIL", Value: req.Git.UserEmail},
		{Name: "SMITH_HANDOFF_PATH", Value: "/smith/handoff/latest.json"},
	}
	env = appendRuntimeCredentialEnv(env, req)
	env = appendGitPolicyEnv(env, req)
	env = appendJournalPolicyEnv(env, req)
	env = appendGitAuthEnv(env, req)
	env = appendSkillMountEnv(env, req.SkillMounts)
	return env
}

func appendSkillMountEnv(env []EnvVar, skillMounts []SkillMount) []EnvVar {
	if len(skillMounts) == 0 {
		return env
	}
	resolvedSkillNames := make([]string, 0, len(skillMounts))
	for _, skill := range skillMounts {
		resolvedSkillNames = append(resolvedSkillNames, skill.Name)
	}
	return append(env,
		EnvVar{Name: "SMITH_SKILL_MOUNT_COUNT", Value: fmt.Sprintf("%d", len(skillMounts))},
		EnvVar{Name: "SMITH_SKILL_MOUNTS", Value: strings.Join(resolvedSkillNames, ",")},
	)
}

const claudeConfigMountPath = "/root/.claude"

func appendRuntimeCredentialEnv(env []EnvVar, req JobRequest) []EnvVar {
	if strings.TrimSpace(req.RuntimeSecretName) != "" {
		env = append(env,
			EnvVar{
				Name: "SMITH_RUNTIME_CREDENTIALS",
				SecretKeyRef: &SecretKeyRef{
					Name: req.RuntimeSecretName,
					Key:  req.RuntimeCredentialsKey,
				},
			},
			EnvVar{
				Name: "OPENAI_API_KEY",
				SecretKeyRef: &SecretKeyRef{
					Name: req.RuntimeSecretName,
					Key:  req.RuntimeCredentialsKey,
				},
			},
		)
		if providerUsesAnthropicKey(req.ProviderID) {
			if strings.TrimSpace(req.ClaudeMaxSecretName) != "" {
				env = append(env, EnvVar{Name: "CLAUDE_CONFIG_DIR", Value: claudeConfigMountPath})
			} else {
				claudeKey := strings.TrimSpace(req.RuntimeCredentialsClaudeKey)
				if claudeKey == "" {
					claudeKey = strings.TrimSpace(req.RuntimeCredentialsKey)
				}
				if claudeKey != "" {
					env = append(env, EnvVar{
						Name: "ANTHROPIC_API_KEY",
						SecretKeyRef: &SecretKeyRef{
							Name: req.RuntimeSecretName,
							Key:  claudeKey,
						},
					})
				}
			}
		}
		return env
	}
	if strings.TrimSpace(req.RuntimeCredentialsValue) != "" {
		env = append(env,
			EnvVar{Name: "SMITH_RUNTIME_CREDENTIALS", Value: req.RuntimeCredentialsValue},
			EnvVar{Name: "OPENAI_API_KEY", Value: req.RuntimeCredentialsValue},
		)
	}
	if providerUsesAnthropicKey(req.ProviderID) {
		if strings.TrimSpace(req.ClaudeMaxSecretName) != "" {
			env = append(env, EnvVar{Name: "CLAUDE_CONFIG_DIR", Value: claudeConfigMountPath})
		} else {
			claudeValue := strings.TrimSpace(req.RuntimeCredentialsClaudeValue)
			if claudeValue == "" {
				claudeValue = strings.TrimSpace(req.RuntimeCredentialsValue)
			}
			if claudeValue != "" {
				env = append(env, EnvVar{Name: "ANTHROPIC_API_KEY", Value: claudeValue})
			}
		}
	}
	return env
}

func providerUsesAnthropicKey(providerID string) bool {
	switch strings.ToLower(strings.TrimSpace(providerID)) {
	case "claude", "anthropic", "claude-max":
		return true
	default:
		return false
	}
}

func appendGitPolicyEnv(env []EnvVar, req JobRequest) []EnvVar {
	if req.GitPolicy == nil {
		return env
	}
	return append(env,
		EnvVar{Name: "SMITH_GIT_POLICY_BRANCH_CLEANUP", Value: string(req.GitPolicy.BranchCleanup)},
		EnvVar{Name: "SMITH_GIT_POLICY_CONFLICT_POLICY", Value: string(req.GitPolicy.ConflictPolicy)},
		EnvVar{Name: "SMITH_GIT_POLICY_DELETE_BRANCH_ON_MERGE", Value: fmt.Sprintf("%t", req.GitPolicy.DeleteBranchOnMerge)},
	)
}

func appendJournalPolicyEnv(env []EnvVar, req JobRequest) []EnvVar {
	if req.JournalPolicy == nil {
		return env
	}
	env = append(env,
		EnvVar{Name: "SMITH_JOURNAL_RETENTION_MODE", Value: string(req.JournalPolicy.RetentionMode)},
		EnvVar{Name: "SMITH_JOURNAL_RETENTION_TTL", Value: req.JournalPolicy.RetentionTTL.String()},
		EnvVar{Name: "SMITH_JOURNAL_ARCHIVE_MODE", Value: string(req.JournalPolicy.ArchiveMode)},
	)
	if strings.TrimSpace(req.JournalPolicy.ArchiveBucket) != "" {
		env = append(env, EnvVar{Name: "SMITH_JOURNAL_ARCHIVE_BUCKET", Value: req.JournalPolicy.ArchiveBucket})
	}
	return env
}

func appendGitAuthEnv(env []EnvVar, req JobRequest) []EnvVar {
	if req.GitAuth == nil {
		return env
	}
	env = append(env, EnvVar{Name: "SMITH_GIT_AUTH_PROVIDER", Value: string(req.GitAuth.Provider)})
	switch req.GitAuth.Provider {
	case GitAuthProviderPAT:
		return append(env, EnvVar{
			Name: "SMITH_GIT_PAT",
			SecretKeyRef: &SecretKeyRef{
				Name: req.GitAuth.PATSecretName,
				Key:  req.GitAuth.PATSecretKey,
			},
		})
	case GitAuthProviderGitHubApp:
		return append(env,
			EnvVar{Name: "SMITH_GITHUB_APP_ID", Value: req.GitAuth.GitHubApp.AppID},
			EnvVar{Name: "SMITH_GITHUB_APP_INSTALLATION_ID", Value: req.GitAuth.GitHubApp.InstallationID},
			EnvVar{
				Name: "SMITH_GITHUB_APP_PRIVATE_KEY",
				SecretKeyRef: &SecretKeyRef{
					Name: req.GitAuth.GitHubApp.PrivateKeySecretName,
					Key:  req.GitAuth.GitHubApp.PrivateKeySecretKey,
				},
			},
		)
	case GitAuthProviderSSH:
		env = append(env, EnvVar{
			Name: "SMITH_GIT_SSH_PRIVATE_KEY",
			SecretKeyRef: &SecretKeyRef{
				Name: req.GitAuth.SSH.PrivateKeySecretName,
				Key:  req.GitAuth.SSH.PrivateKeySecretKey,
			},
		})
		if strings.TrimSpace(req.GitAuth.SSH.KnownHostsSecretName) != "" && strings.TrimSpace(req.GitAuth.SSH.KnownHostsSecretKey) != "" {
			env = append(env, EnvVar{
				Name: "SMITH_GIT_SSH_KNOWN_HOSTS",
				SecretKeyRef: &SecretKeyRef{
					Name: req.GitAuth.SSH.KnownHostsSecretName,
					Key:  req.GitAuth.SSH.KnownHostsSecretKey,
				},
			})
		}
	}
	return env
}

func buildReplicaVolumes(req JobRequest) ([]Volume, []VolumeMount, bool) {
	volumes := []Volume{
		{Name: "workspace", EmptyDir: true},
		{Name: "handoff", ConfigMapName: req.HandoffConfigMapName, Optional: false},
	}
	volumeMounts := []VolumeMount{
		{Name: "workspace", MountPath: "/workspace", ReadOnly: false},
		{Name: "handoff", MountPath: "/smith/handoff", ReadOnly: true},
	}
	hasPRDConfig := strings.TrimSpace(req.PRDConfigMapName) != ""
	if hasPRDConfig {
		volumes = append(volumes, Volume{
			Name:          "workspace-prd",
			ConfigMapName: req.PRDConfigMapName,
			Optional:      false,
		})
	}
	if strings.TrimSpace(req.ClaudeMaxSecretName) != "" {
		volumes = append(volumes, Volume{
			Name:       "claude-config",
			SecretName: req.ClaudeMaxSecretName,
			Items: []KeyToPath{
				{Key: req.ClaudeMaxCredentialsJsonKey, Path: ".credentials.json"},
				{Key: req.ClaudeMaxClaudeJsonKey, Path: ".claude.json"},
				{Key: req.ClaudeMaxSettingsJsonKey, Path: "settings.json"},
			},
		})
		volumeMounts = append(volumeMounts, VolumeMount{
			Name:      "claude-config",
			MountPath: claudeConfigMountPath,
			ReadOnly:  true,
		})
	}
	for i, skill := range req.SkillMounts {
		volumeName := fmt.Sprintf("skill-%d-%s", i, sanitizeName(skill.Name))
		volumes = append(volumes, Volume{
			Name:          volumeName,
			ConfigMapName: skillConfigMapName(skill.Source),
			Optional:      false,
		})
		volumeMounts = append(volumeMounts, VolumeMount{
			Name:      volumeName,
			MountPath: skill.MountPath,
			ReadOnly:  skill.ReadOnly,
		})
	}
	return volumes, volumeMounts, hasPRDConfig
}

func buildReplicaInitContainers(req JobRequest, hasPRDConfig bool) []Container {
	initContainers := []Container{}
	if seedImage := strings.TrimSpace(req.WorkspaceSeedImage); seedImage != "" {
		pullPolicy := strings.TrimSpace(req.WorkspaceSeedPullPolicy)
		if pullPolicy == "" {
			pullPolicy = req.ImagePullPolicy
		}
		initContainers = append(initContainers, Container{
			Name:            "workspace-seed",
			Image:           seedImage,
			ImagePullPolicy: pullPolicy,
			Command: []string{
				"sh",
				"-lc",
				"set -eu; mkdir -p /workspace; if [ -d /seed ]; then cp -a /seed/. /workspace/; fi; chown -R 1000:1000 /workspace",
			},
			VolumeMounts: []VolumeMount{
				{Name: "workspace", MountPath: "/workspace", ReadOnly: false},
			},
		})
	}
	if hasPRDConfig {
		targetPath := workspacePRDAbsolutePath(req.WorkspacePRDPath)
		targetDir := path.Dir(targetPath)
		sourcePath := "/smith/prd/" + strings.TrimSpace(req.PRDConfigMapKey)
		initContainers = append(initContainers, Container{
			Name:            "workspace-prd",
			Image:           req.Image,
			ImagePullPolicy: req.ImagePullPolicy,
			Command: []string{
				"sh",
				"-lc",
				"set -eu; mkdir -p " + shellQuote(targetDir) + "; cp " + shellQuote(sourcePath) + " " + shellQuote(targetPath),
			},
			VolumeMounts: []VolumeMount{
				{Name: "workspace", MountPath: "/workspace", ReadOnly: false},
				{Name: "workspace-prd", MountPath: "/smith/prd", ReadOnly: true},
			},
		})
	}
	return initContainers
}
