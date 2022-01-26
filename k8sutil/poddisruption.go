package k8sutil

import "github.com/yylover/memcached-operator/api/v1alpha1"

func ReconcileRedisPodDisruptionBudget(cr *v1alpha1.RedisCluster, role string) error {
	//podName := cr.ObjectMeta.Name+"-"+role
	return nil
}
