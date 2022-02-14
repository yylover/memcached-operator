package k8sutil

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/yylover/memcached-operator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"net"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"strconv"
	"strings"

	"github.com/go-redis/redis"
)

const (
	RedisPort = ":6379"
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
	csvOutput := csv.NewReader(strings.NewReader(output))
	csvOutput.Comma = ' '
	csvOutput.FieldsPerRecord = -1
	csvOutputRecords, err := csvOutput.ReadAll()
	if err != nil {
		logger.Error(err, "errro parsing Node counts:", "output", output)
	}
	return csvOutputRecords
}

// configureRedisClient 获取pod的redisClient
func configureRedisClient(cr *v1alpha1.RedisCluster, podName string) *redis.Client {
	redisInfo := RedisDetails{
		PodName:   podName,
		Namespace: cr.Namespace,
	}
	var client *redis.Client
	//TODO 密码
	logger := generateRedisManagerLogger(cr.Namespace, cr.Name)
	client = redis.NewClient(&redis.Options{
		Addr:      getRedisServerIP(redisInfo) + ":6379",
		Password:  "",
		DB:        0,
		TLSConfig: nil, // TODO TLSconfig
	})
	logger.Info("getRedisServerIP", "ip:", getRedisServerIP(redisInfo))
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
	if net.ParseIP(redisIP).To4() == nil { // 不是iPv4地址
		redisIP = fmt.Sprintf("[%s]", redisIP)
	}

	logger.Info("success get ip for redis", "ip", redisPod.Status.PodIP, "redisIP:", redisIP)
	return redisIP
}

func generateRedisManagerLogger(namespace, name string) logr.Logger {
	reqLogger := logf.Log.WithName("controller_redis").WithValues("Request.RedisManager.Namespace", namespace, "Request.RedisManager.Name", name)
	return reqLogger
}

// checkRedisNodePresence检查节点是否存在
func checkRedisNodePresence(cr *v1alpha1.RedisCluster, nodeList [][]string, nodeName string) bool {
	logger := generateRedisManagerLogger(cr.Namespace, cr.Name)
	logger.Info("checing if node is in cluster", "node", nodeName)
	for _, node := range nodeList {
		if strings.Contains(node[1], nodeName) {
			return true
		}
	}
	return false
}

// createRedisReplicationCommand
func createRedisReplicationCommand(cr *v1alpha1.RedisCluster, podLeader RedisDetails,
	podFollower RedisDetails) []string {
	logger := generateRedisManagerLogger(cr.Namespace, cr.Name)
	cmd := []string{"redis-cli", "--cluster", "add-node"}
	cmd = append(cmd, getRedisServerIP(podFollower)+RedisPort)
	cmd = append(cmd, getRedisServerIP(podLeader)+RedisPort)
	cmd = append(cmd, "--cluster-slave")

	logger.Info("redis replication create command is :", "command", cmd)
	return cmd
}

// ExecuteRedisReplicationCommand 创建从集群, 不同于主集群的创建，从节点是一个一个加入的
func ExecuteRedisReplicationCommand(cr *v1alpha1.RedisCluster) {
	logger := generateRedisManagerLogger(cr.Namespace, cr.Name)
	replicas := cr.Spec.Size

	nodes := checkRedisCluster(cr)
	for podCount := 0; podCount <= int(*replicas)-1; podCount++ {
		podFollower := RedisDetails{
			PodName:   cr.ObjectMeta.Name + "-follower-" + strconv.Itoa(podCount),
			Namespace: cr.Namespace,
		}
		podLeader := RedisDetails{
			PodName:   cr.ObjectMeta.Name + "-leader-" + strconv.Itoa(podCount),
			Namespace: cr.Namespace,
		}
		podIp := getRedisServerIP(podFollower)
		if !checkRedisNodePresence(cr, nodes, podIp) {
			logger.Info("adding node to cluster : ", "node.ip", podIp, "folloer.pod", podFollower)
			cmd := createRedisReplicationCommand(cr, podLeader, podFollower)
			executeCommand(cr, cmd, cr.ObjectMeta.Name+"-leader-0")
		} else {
			logger.Info("skipping adding node to cluster, already present", "follower.pod", podFollower)
		}
	}
}

// ExecuteRedisClusterCommand 创建redis 集群
func ExecuteRedisClusterCommand(cr *v1alpha1.RedisCluster) {
	logger := generateRedisManagerLogger(cr.Namespace, cr.Name)
	replicas := cr.Spec.Size
	cmd := []string{"redis-cli", "--cluster", "create"}
	for podCount := 0; podCount <= int(*replicas)-1; podCount++ {
		pod := RedisDetails{
			PodName:   cr.ObjectMeta.Name + "-leader-" + strconv.Itoa(podCount),
			Namespace: cr.Namespace,
		}
		cmd = append(cmd, getRedisServerIP(pod)+":6379")
	}
	cmd = append(cmd, "--cluster-yes")

	//TODO 是否密码
	//TODO 是否使用Tls
	logger.Info("RedisCluster creaing cmd :", "Command", cmd)
	//对leader0执行
	executeCommand(cr, cmd, cr.ObjectMeta.Name+"-leader-0")
}

// executeCommand 在pod中执行命令
func executeCommand(cr *v1alpha1.RedisCluster, cmd []string, podName string) {
	logger := generateRedisManagerLogger(cr.Namespace, cr.Name)
	config, err := generateK8sConfig()
	if err != nil {
		logger.Error(err, "cound not find pod to execute")
		return
	}

	//获取容器id
	targetContainner, pod := getContainerID(cr, podName)
	if targetContainner < 0 {
		logger.Error(err, "could not find pod to execute")
		return
	}
	logger.Info("container info ", "podName:", podName, "containerName : ", pod.Spec.Containers[targetContainner].Name)

	req := generateK8sClient().CoreV1().RESTClient().Post().Resource("pods").Name(podName).Namespace(cr.Namespace).SubResource("exec")
	req.VersionedParams(&v1.PodExecOptions{
		Container: pod.Spec.Containers[targetContainner].Name,
		Command:   cmd,
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		logger.Error(err, "failed to init excutor")
		return
	}

	var (
		execOut bytes.Buffer
		execErr bytes.Buffer
	)
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &execOut,
		Stderr: &execErr,
		Tty:    false,
	})
	if err != nil {
		logger.Error(err, "could not exec command", "output", execOut.String(), "Error:", execErr.String())
	}
	logger.Info("Successfully executed the command", "output:", execOut.String())
}

func getContainerID(cr *v1alpha1.RedisCluster, podName string) (int, *v1.Pod) {
	logger := generateRedisManagerLogger(cr.Namespace, cr.Name)
	pod, err := generateK8sClient().CoreV1().Pods(cr.Namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "could not get pods info")
		return -1, nil
	}

	targetContainner := -1
	for containerId, tr := range pod.Spec.Containers {
		logger.Info("Pod Counted successfully", "Count", containerId, "Container Name", tr.Name)
		if tr.Name == cr.ObjectMeta.Name+"-leader" {
			targetContainner = containerId
			break
		}
	}
	return targetContainner, pod
}

// CheckRedisClusterState 检查集群状态
func CheckRedisClusterState(cr *v1alpha1.RedisCluster) int {
	logger := generateRedisManagerLogger(cr.Namespace, cr.Name)
	clusterNode := checkRedisCluster(cr)
	count := 0
	for _, node := range clusterNode {
		if strings.Contains(node[2], "fail") || strings.Contains(node[7], "disconnect") {
			count++
		}
	}

	logger.Info("number of failed nodes in cluster", "fail node count:", count)
	return count
}

// ExecuteFailoverOperation 执行故障切换操作
func ExecuteFailoverOperation(cr *v1alpha1.RedisCluster) {
	executeFailoverCmd(cr, ClusterRoleLeader)
	executeFailoverCmd(cr, ClusterRoleFollower)
}

//executeFailoverCmd 执行故障切换命令
func executeFailoverCmd(cr *v1alpha1.RedisCluster, role string) {
	logger := generateRedisManagerLogger(cr.Namespace, cr.Name)
	replicas := cr.Spec.Size
	podName := cr.Name + "-" + role + "-"
	for podCount := 0; podCount < int(*replicas); podCount++ {
		logger.Info("executing redis failover operation", "Redis Node", podName+strconv.Itoa(podCount))
		client := configureRedisClient(cr, podName+strconv.Itoa(podCount))
		cmd := redis.NewStringCmd("cluster", "reset")
		err := client.Process(cmd)
		if err != nil {
			logger.Error(err, "redis command failed with error")
			flushcommand := redis.NewStringCmd("flushall")
			err := client.Process(flushcommand)
			if err != nil {
				logger.Error(err, "redis flush command failed with this error")
			}
		}

		output, err := cmd.Result()
		if err != nil {
			logger.Error(err, "redis command failed with error:")
		}
		logger.Info("Redis cluster failover executed", "output", output)
	}
}
