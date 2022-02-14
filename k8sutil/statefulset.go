package k8sutil

import (
	"context"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// statefulSetParameters will define statefulsets input params
type statefulSetParameters struct {
	Replicas              *int32
	Metadata              metav1.ObjectMeta
	NodeSelector          map[string]string
	SecurityContext       *corev1.PodSecurityContext
	PriorityClassName     string
	Affinity              *corev1.Affinity
	Tolerations           *[]corev1.Toleration
	EnableMetrics         bool
	PersistentVolumeClaim corev1.PersistentVolumeClaim
	ImagePullSecrets      *[]corev1.LocalObjectReference
	ExternalConfig        *string
}

// containerParameters will define container input params
type containerParameters struct {
	Image              string
	ImagePullPolicy    corev1.PullPolicy
	Resources          *corev1.ResourceRequirements
	PersistenceEnabled *bool
	Role               string
}

func getStatefulLog(namespace, name string) logr.Logger {
	return logf.Log.WithName("controller_redis").WithValues("Request.Stateful.Namespace", namespace, "Request.Stateful.Name", name)
}

//CreateOrUpdateStatefulSet 创建或生成StatefulSet
func CreateOrUpdateStatefulSet(namespace string, stsMeta metav1.ObjectMeta, params statefulSetParameters, ownerDef metav1.OwnerReference, containerParams containerParameters) error {
	logger := getStatefulLog(namespace, stsMeta.Name)
	storedStateful, err := GetStateFulSet(namespace, stsMeta.Name)
	statefulSetDef := generateStateFulSetsDef(stsMeta, params, ownerDef, containerParams)
	if err != nil {
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(statefulSetDef); err != nil {
			logger.Error(err, "Unable to patch redis statefulset with comparison object")
			return err
		}

		if errors.IsNotFound(err) {
			logger.Info("statefulset not find , begin create ")
			return createStatefulSet(namespace, statefulSetDef)
		}
		return err
	}
	logger.Info("Redis statefulSet begin patch")
	return patchStatefulSet(storedStateful, statefulSetDef, namespace)
}

//createStatefulSet 创建redis stateful set
func createStatefulSet(namespace string, stateful *appsv1.StatefulSet) error {
	logger := getStatefulLog(namespace, stateful.Name)
	_, err := generateK8sClient().AppsV1().StatefulSets(namespace).Create(context.TODO(), stateful, metav1.CreateOptions{})
	logger.Info("create statefulset pvc:", "pvc", stateful.Spec.VolumeClaimTemplates)
	if err != nil {
		logger.Error(err, "Redis Stateful create failed")
		return err
	}
	logger.Info("create Redis statefulSet success")
	return nil
}

// updateStatefulSet更新statefulset状态
func updateStatefulSet(namespace string, stateful *appsv1.StatefulSet) error {
	logger := getStatefulLog(namespace, stateful.Name)
	_, err := generateK8sClient().AppsV1().StatefulSets(namespace).Update(context.TODO(), stateful, metav1.UpdateOptions{})
	if err != nil {
		logger.Error(err, "Redis StatefulSet update failed")
		return err
	}
	logger.Info("Redis statefulSet update success")
	return nil
}

// GetStateFulSet 获取Redis StatefulSet
func GetStateFulSet(namespace string, statefulName string) (*appsv1.StatefulSet, error) {
	logger := getStatefulLog(namespace, statefulName)
	statefulSet, err := generateK8sClient().AppsV1().StatefulSets(namespace).Get(context.TODO(), statefulName, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "Redis StatefulSet Get failed")
		return nil, err
	}
	logger.Info("Redis statefulSet Get success")
	return statefulSet, nil
}

//patchStatefulSet patch redis kubenetes statefulSet
func patchStatefulSet(storedStateful *appsv1.StatefulSet, newStateful *appsv1.StatefulSet, namespace string) error {
	logger := getStatefulLog(namespace, storedStateful.Name)
	patchResult, err := patch.DefaultPatchMaker.Calculate(storedStateful, newStateful)
	if err != nil {
		logger.Error(err, "unable to pathc redis statefulSet with comparison object")
	}

	if !patchResult.IsEmpty() {
		logger.Info("changes in statefulset detected, updating")
		newStateful.ResourceVersion = storedStateful.ResourceVersion
		newStateful.CreationTimestamp = storedStateful.CreationTimestamp
		newStateful.ManagedFields = storedStateful.ManagedFields
		newStateful.Spec.VolumeClaimTemplates = storedStateful.Spec.VolumeClaimTemplates
		for k, v := range storedStateful.Annotations {
			if _, present := newStateful.Annotations[k]; !present {
				newStateful.Annotations[k] = v
			}
		}
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(newStateful); err != nil {
			logger.Error(err, "unable to patch redis statefulset with comparison object")
			return err
		}
		return updateStatefulSet(namespace, newStateful)
	}
	return nil
}

// generateStateFulSetsDef 生成Redis的statefulset定义
func generateStateFulSetsDef(stsMeta metav1.ObjectMeta, params statefulSetParameters, ownerDef metav1.OwnerReference, containerParams containerParameters) *appsv1.StatefulSet {
	statefulset := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: stsMeta,
		Spec: appsv1.StatefulSetSpec{
			Selector:    LabelSelectors(stsMeta.GetLabels()),
			ServiceName: stsMeta.Name,
			Replicas:    params.Replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      stsMeta.GetLabels(),
					Annotations: generateStatefulSetsAnots(stsMeta),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            stsMeta.GetName(),
							Image:           containerParams.Image,
							ImagePullPolicy: containerParams.ImagePullPolicy,
							//Env:

						},
					},
					NodeSelector:      params.NodeSelector,
					SecurityContext:   params.SecurityContext,
					PriorityClassName: params.PriorityClassName,
					Affinity:          params.Affinity,
				},
			},
		},
	}
	if containerParams.PersistenceEnabled != nil && *containerParams.PersistenceEnabled {
		statefulset.Spec.VolumeClaimTemplates = append(statefulset.Spec.VolumeClaimTemplates, createPVCTemplate(stsMeta, params.PersistentVolumeClaim))
	}

	AddOwnerRefToObject(statefulset, ownerDef)

	return statefulset
}

func createPVCTemplate(stsMeta metav1.ObjectMeta, storageSpec corev1.PersistentVolumeClaim) corev1.PersistentVolumeClaim {
	pvcTemplate := storageSpec
	pvcTemplate.CreationTimestamp = metav1.Time{}
	pvcTemplate.Name = stsMeta.GetName()
	pvcTemplate.Labels = stsMeta.GetLabels()
	pvcTemplate.Annotations = generateStatefulSetsAnots(stsMeta)
	if storageSpec.Spec.AccessModes == nil {
		pvcTemplate.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	} else {
		pvcTemplate.Spec.AccessModes = storageSpec.Spec.AccessModes
	}
	pvcTemplate.Spec.Resources = storageSpec.Spec.Resources
	pvcTemplate.Spec.Selector = storageSpec.Spec.Selector
	return pvcTemplate
}
