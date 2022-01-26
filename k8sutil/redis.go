package k8sutil

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/yylover/memcached-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"strconv"
	"strings"

	"github.com/go-redis/redis"
)

type RedisDetails struct {
	PodName   string
	Namespace string
}

// CheckRedisNodeCount 获取redis节点的个数
func CheckRedisNodeCount(cr *v1alpha1.RedisCluster, nodeType string) int {
	logger := generateRedisManagerLogger(cr.Namespace, cr.Name)
	clusterNodes := checkRedisCluster(cr)
	count := len(clusterNodes)

	var redisNodeType string
	switch nodeType {
	case ClusterRoleLeader:
		redisNodeType = "master"
	case ClusterRoleFollower:
		redisNodeType = "slave"
	default:
		redisNodeType = nodeType
	}
	if nodeType != "" {
		count := 0
		for _, node := range clusterNodes {
			if strings.Contains(node[2], redisNodeType) {
				count++
			}
		}
		logger.Info("number of redis nodes are", "nodes", strconv.Itoa(count), "type", nodeType)
	} else {
		logger.Info("total number of redis nodes are", "nodes", strconv.Itoa(count))
	}
	return count
}

//checkRedisCluster获取redis集群的节点信息
func checkRedisCluster(cr *v1alpha1.RedisCluster) [][]string {
	var client *redis.Client
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	client = configureRedisClient(cr, cr.ObjectMeta.Name+"-leader-0")
	cmd := redis.NewStringCmd("cluster", "nodes")
	err := client.Process(cmd)
	if err != nil {
		logger.Error(err, "checkRedisCluster get nodes failed")
	}

	output, err := cmd.Result()
	if err != nil {
		logger.Error(err, "checkRedisCluster cmd result failed")
	}
	logger.Info("redis cluster nodes are listed", "output", output)

	return nil
}

// configureRedisClient 获取pod的redisClient
func configureRedisClient(cr *v1alpha1.RedisCluster, podName string) *redis.Client {
	redisInfo := RedisDetails{
		PodName:   podName,
		Namespace: cr.Namespace,
	}
	var client *redis.Client
	//TODO 密码
	client = redis.NewClient(&redis.Options{
		Addr:      getRedisServerIP(redisInfo) + ":6379",
		Password:  "",
		DB:        0,
		TLSConfig: nil, // TODO TLSconfig
	})
	return client
}

//getRedisServerIP 获取redis service的ip
func getRedisServerIP(redisInfo RedisDetails) string {
	logger := generateRedisManagerLogger(redisInfo.Namespace, redisInfo.PodName)

	redisPod, err := generateK8sClient().CoreV1().Pods(redisInfo.Namespace).Get(context.TODO(), redisInfo.PodName, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "getRedisServerIP get pod failed")
	}
	redisIP := redisPod.Status.PodIP
	if net.ParseIP(redisIP).To16() != nil { // ipV6地址
		redisIP = fmt.Sprintf("[%s]", redisIP)
	}

	logger.Info("success get ip for redis", "ip", redisPod.Status.PodIP)
	return redisIP
}

func generateRedisManagerLogger(namespace, name string) logr.Logger {
	reqLogger := logf.Log.WithName("controller_redis").WithValues("Request.RedisManager.Namespace", namespace, "Request.RedisManager.Name", name)
	return reqLogger
}
