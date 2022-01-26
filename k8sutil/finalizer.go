package k8sutil

import (
	"context"
	"github.com/yylover/memcached-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	RedisSingleFinalizer = "redis_finalizer"
)

// HandleRedisFinalizer判断是否删除，是否需要执行finalizer
func HandleRedisFinalizer(single *v1alpha1.RedisSingle, cl client.Client) error {
	//要删除
	if single.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(single, RedisSingleFinalizer) {
			if err := finalizeRedisServices(single); err != nil {
				return err
			}

			if err := finalizeRedisPVC(single); err != nil {
				return err
			}

			controllerutil.RemoveFinalizer(single, RedisSingleFinalizer)
			if err := cl.Update(context.TODO(), single); err != nil {
				return err
			}
		}
	}
	return nil
}

//AddRedisFinalizer 增加finalizer
func AddRedisFinalizer(single *v1alpha1.RedisSingle, cl client.Client) error {
	logger := getStatefulLog(single.Namespace, single.Name)
	if !controllerutil.ContainsFinalizer(single, RedisSingleFinalizer) {
		logger.Info("add redis finalizer success")
		controllerutil.AddFinalizer(single, RedisSingleFinalizer)
		return cl.Update(context.TODO(), single)
	}
	logger.Info("redis finalizer exists")
	return nil
}

// finalizeRedisServices 处理service
func finalizeRedisServices(single *v1alpha1.RedisSingle) error {
	serviceName, headlessServiceName := single.Name, single.Name+"-headless"
	logger := getStatefulLog(single.Namespace, single.Name)
	logger.Info("redis finalizer delete service:", "serviceName", serviceName, "headlessServiceName", headlessServiceName)
	for _, svc := range []string{serviceName, headlessServiceName} {
		err := generateK8sClient().CoreV1().Services(single.Namespace).Delete(context.TODO(), svc, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "Redis finalizer delte serice err :", err.Error())
			return err
		}
	}
	return nil
}

// finalizeRedisPVC 删除RedisPVC
func finalizeRedisPVC(single *v1alpha1.RedisSingle) error {
	PVCName := single.Name + "-" + single.Name + "-0"
	logger := getStatefulLog(single.Namespace, single.Name)
	logger.Info("redis finalizer delete PVC:", "pvcName", PVCName)
	return generateK8sClient().CoreV1().PersistentVolumeClaims(single.Namespace).Delete(context.TODO(), PVCName, metav1.DeleteOptions{})
}
