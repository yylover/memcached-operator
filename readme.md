

## 作为deployment在集群部署
```shell
eval $(minikube docker-env)
docker build -t memcached-op:v1.1  .
k apply -f 
```

## 权限问题
//权限问题
```text
E0128 02:50:15.225060       1 reflector.go:138] pkg/mod/k8s.io/client-go@v0.22.1/tools/cache/reflector.go:167: Failed to watch *v1alpha1.RedisSingle: failed to list *v1alpha1.RedisSingle: redissingles.testop.yylover.com is forbidden: User "system:serviceaccount:develop:default" cannot list resource "redissingles" in API group "testop.yylover.com" at the cluster scope
```



### 原来的redis-operator部署
k apply -f 文件

```shell
k apply -f config/rbac/serviceaccount.yaml
k apply -f config/rbac/role.yaml
k apply -f config/manager/manager.yaml
k apply -f config/rbac/role_binding.yaml
k apply -f example/redis-cluster.yaml
k apply -f example/external_config/configmap.yaml
```

[comment]: <> (创建secret)
k create secret generic redis-secret --from-literal=password=password -n redis-operator

```text
NAME                                 READY   STATUS    RESTARTS   AGE
pod/redis-cluster-follower-0         1/1     Running   0          34m
pod/redis-cluster-follower-1         1/1     Running   0          3m45s
pod/redis-cluster-follower-2         1/1     Running   0          11m
pod/redis-cluster-leader-0           1/1     Running   0          34m
pod/redis-cluster-leader-1           1/1     Running   0          12m
pod/redis-cluster-leader-2           1/1     Running   0          11m
pod/redis-operator-f54b9b555-gxphx   1/1     Running   0          46m

NAME                                      TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)    AGE
service/redis-cluster-follower            ClusterIP   10.99.209.179   <none>        6379/TCP   44m
service/redis-cluster-follower-headless   ClusterIP   None            <none>        6379/TCP   44m
service/redis-cluster-leader              ClusterIP   10.101.127.22   <none>        6379/TCP   44m
service/redis-cluster-leader-headless     ClusterIP   None            <none>        6379/TCP   44m

NAME                             READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/redis-operator   1/1     1            1           46m

NAME                                       DESIRED   CURRENT   READY   AGE
replicaset.apps/redis-operator-f54b9b555   1         1         1       46m

NAME                                      READY   AGE
statefulset.apps/redis-cluster-follower   3/3     44m
statefulset.apps/redis-cluster-leader     3/3     44m
```