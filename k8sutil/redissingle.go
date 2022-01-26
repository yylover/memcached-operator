package k8sutil

import (
	testopv1alpha1 "github.com/yylover/memcached-operator/api/v1alpha1"
)

//CreateSingleRedis will create a singleRedis setup
func CreateSingleRedis(cr *testopv1alpha1.RedisSingle) error {
	logger := getStatefulLog(cr.Namespace, cr.Name)
	logger.Info("CreateSingleRedis begin")

	labes := getRedisLabels(cr.Name, "standalone", "standalone", cr.ObjectMeta.GetLabels())
	annotations := generateStatefulSetsAnots(cr.ObjectMeta)
	objectMetaInfo := generateObjectMetaInformation(cr.ObjectMeta.Name, cr.Namespace, labes, annotations)
	//获取statefulSet
	err := CreateOrUpdateStatefulSet(cr.Namespace, objectMetaInfo, generateRedisStandaloneParams(cr), redisAsOwner(cr), generateRedisStandaloneContainerParams(cr))
	if err != nil {
		logger.Error(err, "cannot create single Redis")
		return err
	}
	return nil
}

// CreateSingleRedisService创建redis单例的service
func CreateSingleRedisService(cr *testopv1alpha1.RedisSingle) error {
	logger := serviceLogger(cr.Namespace, cr.Name)
	labels := getRedisLabels(cr.Name, "standalone", "standalone", cr.Labels)
	annotations := generateServiceAnots(cr.ObjectMeta)
	//TODO Exporter

	headlessObjectMetaInfo := generateObjectMetaInformation(cr.Name+"-headless", cr.Namespace, labels, annotations)
	err := CreateOrUpdateHeadlessService(cr.Namespace, headlessObjectMetaInfo, redisAsOwner(cr))
	if err != nil {
		logger.Error(err, "cannot create standalone headless service for redis")
		return err
	}

	objectMetaInfo := generateObjectMetaInformation(cr.Name, cr.Namespace, labels, annotations)
	err = CreateOrUpdateService(cr.Namespace, objectMetaInfo, redisAsOwner(cr))
	if err != nil {
		logger.Error(err, "cannot create standalone service for redis")
	}

	return nil
}

// generateRedisStandaloneParams 生成redis单例相关信息
func generateRedisStandaloneParams(cr *testopv1alpha1.RedisSingle) statefulSetParameters {
	replicas := int32(1)
	res := statefulSetParameters{
		Replicas: &replicas,
	}

	if cr.Spec.Storage != nil {
		res.PersistentVolumeClaim = cr.Spec.Storage.VolumeClaimTemplate
	}
	if cr.Spec.RedisConfig != nil {
		res.ExternalConfig = cr.Spec.RedisConfig.AdditionalRedisConfig
	}

	//enable export
	return res
}

// generateRedisStandaloneContainerParams 生成redis容器信息
func generateRedisStandaloneContainerParams(cr *testopv1alpha1.RedisSingle) containerParameters {
	trueProperty := true
	containerParams := containerParameters{
		Image:           cr.Spec.KubernetesConfig.Image,
		ImagePullPolicy: cr.Spec.KubernetesConfig.ImagePullPolicy,
		Resources:       cr.Spec.KubernetesConfig.Resource,
	}

	if cr.Spec.Storage != nil {
		containerParams.PersistenceEnabled = &trueProperty
	}
	return containerParams
}
