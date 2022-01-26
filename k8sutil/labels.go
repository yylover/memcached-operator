package k8sutil

import (
	"github.com/yylover/memcached-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LabelSelectors generates object for label selection
func LabelSelectors(labels map[string]string) *metav1.LabelSelector {
	return &metav1.LabelSelector{MatchLabels: labels}
}

// generateTypeMeta generates the meta information
func generateTypeMeta(resourceKind string, apiVersion string) metav1.TypeMeta {
	return metav1.TypeMeta{
		Kind:       resourceKind,
		APIVersion: apiVersion,
	}
}

func generateStatefulSetsAnots(stsMeta metav1.ObjectMeta) map[string]string {
	anots := map[string]string{
		"redis.yylover.in":       "true",
		"redis.yylover.instance": stsMeta.GetName(),
	}
	for k, v := range stsMeta.GetAnnotations() {
		anots[k] = v
	}
	return filterAnnotations(anots)
}

func generateServiceAnots(stsMeta metav1.ObjectMeta) map[string]string {
	anots := map[string]string{
		"redis.yylover.in":       "true",
		"redis.yylover.instance": stsMeta.GetName(),
	}
	for k, v := range stsMeta.GetAnnotations() {
		anots[k] = v
	}
	return filterAnnotations(anots)
}

//删除自动生成的
func filterAnnotations(anots map[string]string) map[string]string {
	delete(anots, "kubectl.kubernetes.io/last-applied-configuration")
	delete(anots, "banzaicloud.com/last-applied")
	return anots
}

func getRedisLabels(name, setupType, role string, labels map[string]string) map[string]string {
	lbls := map[string]string{
		"app":              name,
		"redis_setup_type": setupType,
		"role":             role,
	}
	for k, v := range labels {
		lbls[k] = v
	}
	return lbls
}

// generateObjectMetaInformation生成对象meta信息
func generateObjectMetaInformation(name string, namespace string, labels map[string]string, annotations map[string]string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        name,
		Namespace:   namespace,
		Labels:      labels,
		Annotations: annotations,
	}
}

// redisAsOwner生成对象引用
func redisAsOwner(cr *v1alpha1.RedisSingle) metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: cr.APIVersion,
		Kind:       cr.Kind,
		Name:       cr.Name,
		UID:        cr.UID,
		Controller: &trueVar,
	}
}

// redisClusterAsOwner生成对象引用
func redisClusterAsOwner(cr *v1alpha1.RedisCluster) metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: cr.APIVersion,
		Kind:       cr.Kind,
		Name:       cr.Name,
		UID:        cr.UID,
		Controller: &trueVar,
	}
}

// AddOwnerRefToObject add
func AddOwnerRefToObject(obj metav1.Object, ownerRef metav1.OwnerReference) {
	obj.SetOwnerReferences(append(obj.GetOwnerReferences(), ownerRef))
}
