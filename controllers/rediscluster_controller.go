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
	errors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	testopv1alpha1 "github.com/yylover/memcached-operator/api/v1alpha1"
)

// RedisClusterReconciler reconciles a RedisCluster object
type RedisClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=testop.yylover.com,resources=redisclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=testop.yylover.com,resources=redisclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=testop.yylover.com,resources=redisclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments;statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services;persistentvolumeclaims;pods;pods/exec,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RedisCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.Reconciler error
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *RedisClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = ctrllog.FromContext(ctx)
	log := ctrllog.FromContext(ctx)
	log.Info("redis-cluster newcomming")

	instance := &testopv1alpha1.RedisCluster{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Error(err, "RedisCluster instance get failed ")
			return ctrl.Result{}, nil
		}
	}

	if err := controllerutil.SetControllerReference(instance, instance, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	//创建leader
	err = k8sutil.CreateRedisLeader(instance)
	if err != nil {
		log.Error(err, "CreateRedisLeader failed")
		return ctrl.Result{}, err
	}
	if instance.Spec.RedisLeader.Replicas != nil && *instance.Spec.RedisLeader.Replicas != 0 {
		err = k8sutil.CreateRedisLeaderService(instance)
		if err != nil {
			log.Error(err, "CreateRedisLeaderService failed")
			return ctrl.Result{}, nil
		}
	}

	//创建follower
	err = k8sutil.CreateRedisFollower(instance)
	if err != nil {
		log.Error(err, "CreateRedisLeader failed")
		return ctrl.Result{}, err
	}
	if instance.Spec.RedisFollower.Replicas != nil && *instance.Spec.RedisFollower.Replicas != 0 {
		err = k8sutil.CreateRedisFollowerService(instance)
		if err != nil {
			log.Error(err, "CreateRedisFollowerService failed")
			return ctrl.Result{}, nil
		}
	}

	redisLeaderSet, err := k8sutil.GetStateFulSet(instance.Namespace, instance.ObjectMeta.Name+"-leader")
	if err != nil {
		return ctrl.Result{}, err
	}
	redisFollowerSet, err := k8sutil.GetStateFulSet(instance.Namespace, instance.ObjectMeta.Name+"-follower")
	if err != nil {
		return ctrl.Result{}, err
	}
	leaderReplicas := instance.Spec.RedisLeader.Replicas
	if leaderReplicas == nil {
		leaderReplicas = instance.Spec.Size
	}
	followerReplicas := instance.Spec.RedisFollower.Replicas
	if followerReplicas == nil {
		followerReplicas = instance.Spec.Size
	}
	totalReplicas := *leaderReplicas + *followerReplicas
	log.Info("replicas : ", "total:", totalReplicas, "leaderReplicas", *leaderReplicas, "followerReplicas:", *followerReplicas)
	if int(redisLeaderSet.Status.ReadyReplicas) != int(*leaderReplicas) && int(redisFollowerSet.Status.ReadyReplicas) != int(*followerReplicas) {
		log.Info("replicas size error:")
		return ctrl.Result{RequeueAfter: time.Second * 30}, nil
	}

	log.Info("create reader cluster by execting cluster creation commands")
	if k8sutil.CheckRedisNodeCount(instance, "") != int(totalReplicas) {
		leaderCount := k8sutil.CheckRedisNodeCount(instance, "leader")
		log.Info("CheckRedisNodeCount lead : ", "leaderCount", leaderCount)
		if leaderCount != int(*leaderReplicas) {
			log.Info("not all leader are part of the cluster ...", "leaders.Count", leaderCount, "instance.Size", *leaderReplicas)
			k8sutil.ExecuteRedisClusterCommand(instance)
		} else {
			if *followerReplicas > 0 {
				k8sutil.ExecuteRedisReplicationCommand(instance)
			} else {
				log.Info("no follower/replicas configured, skipping replication configuration", "leaderCOunt:", *leaderReplicas, "followerCOunt:", *followerReplicas)
			}
		}
	} else {
		log.Info("redis leader count is desired, check redis cluster status")
		if k8sutil.CheckRedisClusterState(instance) > 0 {
			k8sutil.ExecuteFailoverOperation(instance)
		}
	}

	return ctrl.Result{RequeueAfter: time.Second * 20}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&testopv1alpha1.RedisCluster{}).
		Complete(r)
}
