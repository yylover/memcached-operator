package k8sutil

import (
	"context"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	redisPort = 6379
)

func serviceLogger(namespace string, name string) logr.Logger {
	return logf.Log.WithName("controller_redis").WithValues("Request.Service.Namespace", namespace, "Request.Service.Name", name)
}

// CreateOrUpdateHeadlessService method will create or update Redis headless service
func CreateOrUpdateHeadlessService(namespace string, serviceMeta metav1.ObjectMeta, ownerDef metav1.OwnerReference) error {
	logger := serviceLogger(namespace, serviceMeta.Name)
	storedService, err := getService(namespace, serviceMeta.Name)
	serviceDef := generateHeadlessServiceDef(serviceMeta, ownerDef)
	if err != nil {
		if errors.IsNotFound(err) {
			//set last annotation
			if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(serviceDef); err != nil {
				logger.Error(err, "unable to patch redis service with comparison object")
				return err
			}

			return createService(namespace, serviceDef)
		}
		return err
	}

	return patchService(storedService, serviceDef, namespace)
}

func CreateOrUpdateService(namespace string, serviceMeta metav1.ObjectMeta, ownerRef metav1.OwnerReference) error {
	logger := serviceLogger(namespace, serviceMeta.Name)
	serviceDef := generateServiceDef(serviceMeta, serviceMeta.Labels, ownerRef)
	storedService, err := getService(namespace, serviceMeta.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(serviceDef); err != nil {
				logger.Error(err, "unable to patch redis service with compare annotations")
			}
			return createService(namespace, serviceDef)
		}
		return err
	}
	return patchService(storedService, serviceDef, namespace)
}

// getService 获取service
func getService(namespace string, serviceName string) (*corev1.Service, error) {
	logger := serviceLogger(namespace, serviceName)
	serviceInfo, err := generateK8sClient().CoreV1().Services(namespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "Redis service get failed")
		return nil, err
	}
	logger.Info("Redis service get success")
	return serviceInfo, nil
}

// updateService 更新service
func updateService(namespace string, service *corev1.Service) error {
	logger := serviceLogger(namespace, service.Name)
	_, err := generateK8sClient().CoreV1().Services(namespace).Update(context.TODO(), service, metav1.UpdateOptions{})
	if err != nil {
		logger.Error(err, "Redis service update failed")
		return err
	}
	logger.Info("Redis service update success")
	return nil
}

// createService 创建service
func createService(namespace string, service *corev1.Service) error {
	logger := serviceLogger(namespace, service.Name)
	_, err := generateK8sClient().CoreV1().Services(namespace).Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		logger.Error(err, "Redis service create failed")
		return err
	}
	logger.Info("Redis service create success")
	return nil
}

// patchService
func patchService(storedService *corev1.Service, newService *corev1.Service, namespace string) error {
	logger := serviceLogger(namespace, storedService.Name)
	patchResult, err := patch.DefaultPatchMaker.Calculate(storedService, newService, patch.IgnoreStatusFields())
	if err != nil {
		logger.Error(err, "unable to patch redis with comparison object")
		return err
	}

	if !patchResult.IsEmpty() {
		newService.Spec.ClusterIP = storedService.Spec.ClusterIP
		newService.ResourceVersion = storedService.ResourceVersion
		newService.CreationTimestamp = storedService.CreationTimestamp
		newService.ManagedFields = storedService.ManagedFields
		for key, value := range storedService.Annotations {
			if _, present := newService.Annotations[key]; !present {
				newService.Annotations[key] = value
			}
		}
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(newService); err != nil {
			logger.Error(err, "unable to patch redis service with comparison object")
			return err
		}

		logger.Info("syncing redis service with defined properties")
		return updateService(namespace, newService)
	}

	logger.Info("redis service is already in-sync")
	return nil
}

func generateServiceType(k8sServiceType string) corev1.ServiceType {
	var serviceType corev1.ServiceType
	switch k8sServiceType {
	case "LoadBalancer":
		serviceType = corev1.ServiceTypeLoadBalancer
	case "NodePort":
		serviceType = corev1.ServiceTypeNodePort
	case "ClusterIP":
		serviceType = corev1.ServiceTypeClusterIP
	default:
		serviceType = corev1.ServiceTypeClusterIP
	}
	return serviceType
}

func generateHeadlessServiceDef(serviceMeta metav1.ObjectMeta, ownerDef metav1.OwnerReference) *corev1.Service {
	service := &corev1.Service{
		TypeMeta:   generateTypeMeta("Service", "core/v1"),
		ObjectMeta: serviceMeta,
		Spec: corev1.ServiceSpec{
			ClusterIP: "None", //表明是无头service
			Selector:  serviceMeta.Labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "redis-client",
					Port:       redisPort,
					TargetPort: intstr.FromInt(redisPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	AddOwnerRefToObject(service, ownerDef)
	return service
}

func generateServiceDef(serviceMeta metav1.ObjectMeta, labels map[string]string, ownerRef metav1.OwnerReference) *corev1.Service {
	service := &corev1.Service{
		TypeMeta:   generateTypeMeta("service", "core/v1"),
		ObjectMeta: serviceMeta,
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "redis-client",
					Port:       redisPort,
					TargetPort: intstr.FromInt(redisPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	AddOwnerRefToObject(service, ownerRef)
	return service
}
