/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"reflect"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"strings"

	testopv1alpha1 "github.com/yylover/memcached-operator/api/v1alpha1"
)

// MemcachedReconciler reconciles a Memcached object
type MemcachedReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//用于生成rbac访问控制
//+kubebuilder:rbac:groups=testop.yylover.com,resources=memcacheds,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=testop.yylover.com,resources=memcacheds/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=testop.yylover.com,resources=memcacheds/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Memcached object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *MemcachedReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx)
	log.Info("new comming : ", "namespace", req.Namespace, "req.name", req.Name)
	if true {
		//return ctrl.Result{}, nil
	}

	// your logic here
	memcached := &testopv1alpha1.Memcached{}
	err := r.Client.Get(ctx, req.NamespacedName, memcached)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("memcached resource not found, ignoring since object must be delete ")
			// 有可能对象已经被删掉了，或者被清理掉了,直接返回不用处理
			return ctrl.Result{}, nil
		}
		log.Error(err, "get memcached crd failed :")
		//重新执行reconcile
		return ctrl.Result{}, err
	}

	log.Info("memcached spec :", "spec.size", memcached.Spec.Size, "status.nodes", strings.Join(memcached.Status.Nodes, "|"))
	deployment := &appsv1.Deployment{}
	err = r.Client.Get(ctx, req.NamespacedName, deployment)
	if err != nil {
		//create one
		if errors.IsNotFound(err) {
			dep := r.deploymentForMemcached(memcached)
			log.Info("creating a new deployment", "deploy.namespace", dep.Namespace)
			err1 := r.Client.Create(ctx, dep)
			if err1 != nil {
				log.Error(err1, "create deployment failed")
			}
			//重新调度一次
			log.Info("create deployment success， requeued ")
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(err, "failed to get Deployment")
		return ctrl.Result{}, err
	}

	//检查deployment的size是否跟spec相等（deployment或statefulset会自动保证相等，这里为什么还需要判断呢）
	size := memcached.Spec.Size
	if *deployment.Spec.Replicas != size {
		log.Info("memcached size not equal : ", "memcached.size", size, "deployment.size", *deployment.Spec.Replicas)
		err = r.Update(ctx, deployment)
		if err != nil {
			log.Error(err, "failed to update deployment")
			return ctrl.Result{}, err
		}

		//10s后重试
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	//检查memcached的Status和Pods name
	podList := &corev1.PodList{}
	//client.ListOptions{}
	err = r.List(ctx, podList, client.InNamespace(memcached.Namespace), client.MatchingLabels(getLabels(memcached)))
	if err != nil {
		log.Error(err, "list pods failed : ")
		return ctrl.Result{}, err
	}

	podNames := []string{}
	for _, item := range podList.Items {
		podNames = append(podNames, item.Name)
	}
	log.Info("podsNames : ", "podnames :", strings.Join(podNames, "|"), "status.nodes", strings.Join(memcached.Status.Nodes, "|"))
	if !reflect.DeepEqual(podNames, memcached.Status.Nodes) {
		memcached.Status.Nodes = podNames
		err := r.Status().Update(ctx, memcached)
		if err != nil {
			log.Error(err, "update status failed")
			return ctrl.Result{}, err
		}
		log.Info("update status success")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MemcachedReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&testopv1alpha1.Memcached{}).WithOptions(controller.Options{MaxConcurrentReconciles: 1}).Owns(&appsv1.Deployment{}).
		Complete(r)
	// Owns(&appsv1.Deployment{})关注deployment以后，deployment的操作和Pods的操作都会被事件监听
}

func getLabels(m *testopv1alpha1.Memcached) map[string]string {
	return map[string]string{
		"app":          "memcached",
		"memcached_cr": m.Name, // CR name
	}
}

func (r *MemcachedReconciler) deploymentForMemcached(m *testopv1alpha1.Memcached) *appsv1.Deployment {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &m.Spec.Size,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":          "memcached",
					"memcached_cr": m.Name, // CR name
				},
			},
			Template: corev1.PodTemplateSpec{ //pod的Template配置
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":          "memcached",
						"memcached_cr": m.Name, // CR name
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: "memcached:1.6-alpine",
							Name:  "memcached",
							//Command: []string{"memcached", "-m=64", "-o", "modern", "-v"},
							Ports: []corev1.ContainerPort{{
								ContainerPort: 11211,
								Name:          "memcached",
							}},
						},
					},
				},
			},
		},
	}

	ctrl.SetControllerReference(m, dep, r.Scheme)
	return dep
}
