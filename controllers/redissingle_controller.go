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
	"github.com/yylover/memcached-operator/k8sutil"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	testopv1alpha1 "github.com/yylover/memcached-operator/api/v1alpha1"
)

// RedisSingleReconciler reconciles a RedisSingle object
type RedisSingleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=testop.yylover.com,resources=redissingles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=testop.yylover.com,resources=redissingles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=testop.yylover.com,resources=redissingles/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RedisSingle object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *RedisSingleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("redis-single newcomming")
	// your logic here
	redis := &testopv1alpha1.RedisSingle{}
	err := r.Client.Get(ctx, req.NamespacedName, redis)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("redis-single object cannot find")
			return ctrl.Result{}, nil
		}
		log.Error(err, "get redis-single object failed")
		return ctrl.Result{}, err
	}
	//判断资源是否删除，删除的话要清掉相关的finalizer?
	//handle finalizer
	if err := k8sutil.HandleRedisFinalizer(redis, r.Client); err != nil {
		return ctrl.Result{}, err
	}

	if err := k8sutil.AddRedisFinalizer(redis, r.Client); err != nil {
		return ctrl.Result{}, err
	}

	if err := ctrl.SetControllerReference(redis, redis, r.Scheme); err != nil {
		log.Error(err, "SetControllerReference failed")
		return ctrl.Result{}, err
	}

	//创建statefulSet
	err = k8sutil.CreateSingleRedis(redis)
	if err != nil {
		return ctrl.Result{}, err
	}

	//创建headless service
	err = k8sutil.CreateSingleRedisService(redis)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: time.Second * 15}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisSingleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&testopv1alpha1.RedisSingle{}).
		Complete(r)
}
