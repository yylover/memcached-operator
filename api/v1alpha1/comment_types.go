package v1alpha1

import corev1 "k8s.io/api/core/v1"

type KubernetesConfig struct {
	Image           string                       `json:"image"`
	ImagePullPolicy corev1.PullPolicy            `json:"imagePullPolicy,omitempty"`
	Resource        *corev1.ResourceRequirements `json:"resources,omitempty"`
}

type RedisConfig struct {
	AdditionalRedisConfig *string `json:"additionalRedisConfig,omitempty"`
}

type Storage struct {
	VolumeClaimTemplate corev1.PersistentVolumeClaim `json:"volumeClaimTemplate,omitempty"`
}
