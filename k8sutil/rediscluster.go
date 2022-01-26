package k8sutil

import (
	"github.com/yylover/memcached-operator/api/v1alpha1"
)

const (
	ClusterRoleLeader   = "leader"
	ClusterRoleFollower = "follower"
)

// CreateRedisLeader 创建leader redis设置
func CreateRedisLeader(cr *v1alpha1.RedisCluster) error {
	prop := RedisClusterSTS{
		RedisStatefulSetType: ClusterRoleLeader,
	}
	if cr.Spec.RedisLeader.RedisConfig != nil {
		prop.ExternalConfig = cr.Spec.RedisLeader.RedisConfig.AdditionalRedisConfig
	}
	return prop.CreateRedisClusterSetup(cr)
}

//CreateRedisFollower 创建RedisCluster follower
func CreateRedisFollower(cr *v1alpha1.RedisCluster) error {
	prop := RedisClusterSTS{
		RedisStatefulSetType: ClusterRoleFollower,
	}
	if cr.Spec.RedisLeader.RedisConfig != nil {
		prop.ExternalConfig = cr.Spec.RedisLeader.RedisConfig.AdditionalRedisConfig
	}
	return prop.CreateRedisClusterSetup(cr)
}

//CreateRedisLeaderService 创建RedisCluster leader service
func CreateRedisLeaderService(cr *v1alpha1.RedisCluster) error {
	prop := RedisCluterService{
		RedisServiceRole: ClusterRoleLeader,
	}

	return prop.CreateRedisClusterServcie(cr)
}

//CreateRedisFollowerService 创建RedisCluster follower service
func CreateRedisFollowerService(cr *v1alpha1.RedisCluster) error {
	prop := RedisCluterService{
		RedisServiceRole: ClusterRoleFollower,
	}

	return prop.CreateRedisClusterServcie(cr)
}

//RedisClusterSTS 是调用Redis stateful函数的接口
type RedisClusterSTS struct {
	RedisStatefulSetType string // leader follower
	ExternalConfig       *string
}

// RedisCluterService 是调用Redis service的接口
type RedisCluterService struct {
	RedisServiceRole string
}

//generateRedisClusterParams 生成Redis集群stateful参数
func generateRedisClusterParams(cr *v1alpha1.RedisCluster, replicas *int32, externalConfig *string) statefulSetParameters {
	res := statefulSetParameters{
		Metadata:     cr.ObjectMeta,
		Replicas:     replicas,
		NodeSelector: cr.Spec.NodeSelector,
	}
	if cr.Spec.Storage != nil {
		res.PersistentVolumeClaim = cr.Spec.Storage.VolumeClaimTemplate
	}
	if externalConfig != nil {
		res.ExternalConfig = externalConfig
	}
	return res
}

//generateRedisClusterContainerParams
func generateRedisClusterContainerParams(cr *v1alpha1.RedisCluster) containerParameters {
	trueProperty := true
	res := containerParameters{
		Role:            "cluster",
		Image:           cr.Spec.KubernetesConfig.Image,
		ImagePullPolicy: cr.Spec.KubernetesConfig.ImagePullPolicy,
		Resources:       cr.Spec.KubernetesConfig.Resource,
	}

	if cr.Spec.Storage != nil {
		res.PersistenceEnabled = &trueProperty
	}

	return res
}

// CreateRedisClusterSetup redis集群设置
func (service RedisClusterSTS) CreateRedisClusterSetup(cr *v1alpha1.RedisCluster) error {
	statefulName := cr.ObjectMeta.Name + "-" + service.RedisStatefulSetType
	logger := getStatefulLog(cr.Namespace, statefulName)
	labels := getRedisLabels(statefulName, "cluster", service.RedisStatefulSetType, cr.ObjectMeta.GetLabels())
	annotations := generateStatefulSetsAnots(cr.ObjectMeta)
	objectMetaInfo := generateObjectMetaInformation(statefulName, cr.Namespace, labels, annotations)
	err := CreateOrUpdateStatefulSet(
		cr.Namespace, objectMetaInfo, generateRedisClusterParams(cr, service.getReplicaCount(cr), service.ExternalConfig), redisClusterAsOwner(cr), generateRedisClusterContainerParams(cr))
	if err != nil {
		logger.Error(err, "RedisCluster create failed")
		return err
	}
	return nil
}

// getReplicaCount 获取集群数量配置
func (service RedisClusterSTS) getReplicaCount(cr *v1alpha1.RedisCluster) *int32 {
	var replicas *int32
	if service.RedisStatefulSetType == ClusterRoleLeader {
		replicas = cr.Spec.RedisLeader.Replicas
	} else {
		replicas = cr.Spec.RedisFollower.Replicas
	}
	if replicas == nil {
		replicas = cr.Spec.Size
	}

	return replicas
}

// CreateRedisClusterServcie 生成Redis集群的Service
func (service RedisCluterService) CreateRedisClusterServcie(cr *v1alpha1.RedisCluster) error {
	serviceName := cr.ObjectMeta.Name + "-" + service.RedisServiceRole
	logger := serviceLogger(cr.Namespace, serviceName)
	labels := getRedisLabels(serviceName, "cluster", service.RedisServiceRole, cr.ObjectMeta.Labels)
	annotations := generateServiceAnots(cr.ObjectMeta)

	headlessObjectMetaInfo := generateObjectMetaInformation(serviceName+"-headless", cr.Namespace, labels, annotations)
	err := CreateOrUpdateHeadlessService(cr.Namespace, headlessObjectMetaInfo, redisClusterAsOwner(cr))
	if err != nil {
		logger.Error(err, "RedisCluster create headless service failed", "setup.Type", service.RedisServiceRole)
		return err
	}

	objectMetaInfo := generateObjectMetaInformation(serviceName, cr.Namespace, labels, annotations)
	err = CreateOrUpdateService(cr.Namespace, objectMetaInfo, redisClusterAsOwner(cr))
	if err != nil {
		logger.Error(err, "RedisCluster create service failed", "setup.Type", service.RedisServiceRole)
		return err
	}
	return nil
}
