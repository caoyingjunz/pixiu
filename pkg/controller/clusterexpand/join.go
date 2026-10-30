/*
Copyright 2026 The Pixiu Authors.

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

package clusterexpand

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"

	"github.com/caoyingjunz/pixiu/pkg/client"
	clustercontroller "github.com/caoyingjunz/pixiu/pkg/controller/cluster"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	kubeadmConfigMapName = "kubeadm-config"
	kubeadmConfigDataKey = "ClusterConfiguration"
	bootstrapTokenTTL    = 2 * time.Hour
)

type kubeadmClusterConfiguration struct {
	KubernetesVersion    string `json:"kubernetesVersion"`
	ImageRepository      string `json:"imageRepository"`
	ControlPlaneEndpoint string `json:"controlPlaneEndpoint"`
}

// clusterInfo 集群扩容能力探测的中间结果。
type clusterInfo struct {
	Config  *types.ClusterExpansionConfig
	Client  *client.ClusterSet
	Kubeadm *kubeadmClusterConfiguration
	Server  string
	CAData  []byte
}

type joinConfig struct {
	APIServer       string
	Token           string
	CAHash          string
	KubeRelease     string
	ImageRepository string
}

// detectCluster 探测集群是否具备扩容能力，并返回 join 所需的原始信息。
// 不可扩容时不返回 error，而是把原因写入 Config.Reason，便于前端直接展示。
func (c *controller) detectCluster(ctx context.Context, obj *model.Cluster) *clusterInfo {
	info := &clusterInfo{
		Config: &types.ClusterExpansionConfig{},
	}

	opts := clustercontroller.ResolveClusterSetOptions(ctx, c.factory, obj)
	cs, err := client.NewClusterSetWithOptions(obj.KubeConfig, opts)
	if err != nil {
		info.Config.Reason = fmt.Sprintf("加载集群 kubeconfig 失败: %v", err)
		return info
	}
	info.Client = cs

	// 1) kubeadm 体系判定：kubez add-node 本质是 kubeadm join
	cm, err := cs.Client.CoreV1().ConfigMaps("kube-system").Get(ctx, kubeadmConfigMapName, metav1.GetOptions{})
	if err != nil {
		info.Config.Reason = "未检测到 kube-system/kubeadm-config，当前集群不是 kubeadm 体系，暂不支持扩容"
		return info
	}
	var kubeadmCfg kubeadmClusterConfiguration
	if err = yaml.Unmarshal([]byte(cm.Data[kubeadmConfigDataKey]), &kubeadmCfg); err != nil {
		info.Config.Reason = fmt.Sprintf("解析 kubeadm-config 失败: %v", err)
		return info
	}
	info.Kubeadm = &kubeadmCfg

	// 2) kubeconfig 中的 apiserver 与 CA
	rawKubeconfig, err := client.ParseKubeConfigBytes(obj.KubeConfig)
	if err != nil {
		info.Config.Reason = fmt.Sprintf("解析集群 kubeconfig 失败: %v", err)
		return info
	}
	kubeconfig, err := clientcmd.Load(rawKubeconfig)
	if err != nil {
		info.Config.Reason = fmt.Sprintf("加载集群 kubeconfig 失败: %v", err)
		return info
	}
	if apiCtx := kubeconfig.Contexts[kubeconfig.CurrentContext]; apiCtx != nil {
		if clusterCfg := kubeconfig.Clusters[apiCtx.Cluster]; clusterCfg != nil {
			info.Server = clusterCfg.Server
			info.CAData = clusterCfg.CertificateAuthorityData
		}
	}

	// 3) 运行时探测：仅 containerd
	nodes, err := cs.Client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		info.Config.Reason = fmt.Sprintf("查询集群节点失败: %v", err)
		return info
	}
	if len(nodes.Items) == 0 {
		info.Config.Reason = "集群暂无可探测的节点，无法确认容器运行时"
		return info
	}
	hasContainerd := false
	hasDocker := false
	for i := range nodes.Items {
		version := nodes.Items[i].Status.NodeInfo.ContainerRuntimeVersion
		if strings.HasPrefix(version, "containerd://") {
			hasContainerd = true
		}
		if strings.HasPrefix(version, "docker://") {
			hasDocker = true
		}
	}
	if !hasContainerd || hasDocker {
		info.Config.Reason = "当前集群容器运行时不是纯 containerd，暂不支持扩容（kubez add-node 仅支持 containerd）"
		return info
	}

	// 4) 版本 / 镜像仓库 / 控制面地址
	version := strings.TrimPrefix(strings.TrimSpace(kubeadmCfg.KubernetesVersion), "v")
	if version == "" {
		if sv, verr := cs.Client.Discovery().ServerVersion(); verr == nil {
			version = strings.TrimPrefix(sv.GitVersion, "v")
		}
	}
	info.Config.KubernetesVersion = version
	info.Config.ImageRepository = strings.TrimSpace(kubeadmCfg.ImageRepository)
	info.Config.ControlPlaneEndpoint = strings.TrimSpace(kubeadmCfg.ControlPlaneEndpoint)
	info.Config.Runtime = string(model.ContainerdCRI)

	// 5) 计划存在时仅作为默认值（不参与执行依赖）
	info.Config.DefaultNetworkInterface = "eth0"
	if obj.PlanId > 0 {
		if cfg, cerr := c.factory.Plan().Config().GetByPlan(ctx, obj.PlanId); cerr == nil && cfg != nil {
			info.Config.DefaultOSImage = cfg.OSImage
			networkSpec := types.NetworkSpec{}
			if networkSpec.Unmarshal(cfg.Network) == nil && networkSpec.NetworkInterface != "" {
				info.Config.DefaultNetworkInterface = networkSpec.NetworkInterface
			}
		}
	}

	info.Config.Supported = true
	return info
}

// resolveJoin 计算 join 所需的参数：优先请求覆盖，其次 kubeadm-config，最后 kubeconfig。
func (c *controller) resolveJoin(ctx context.Context, obj *model.Cluster, info *clusterInfo, options *types.ExpansionOptions) (*joinConfig, error) {
	if info == nil || info.Config == nil || !info.Config.Supported {
		reason := "集群不支持扩容"
		if info != nil && info.Config != nil && info.Config.Reason != "" {
			reason = info.Config.Reason
		}
		return nil, fmt.Errorf("%s", reason)
	}

	apiServer := strings.TrimSpace(options.APIServer)
	if apiServer == "" && info.Kubeadm != nil {
		apiServer = strings.TrimSpace(info.Kubeadm.ControlPlaneEndpoint)
	}
	if apiServer == "" {
		apiServer = info.Server
	}
	apiServer, err := normalizeAPIServer(apiServer)
	if err != nil {
		return nil, err
	}

	// kubez 会自动补 "sha256:" 前缀，这里容忍用户粘贴完整 join 命令中的带前缀值
	caHash := strings.TrimPrefix(strings.TrimSpace(options.CAHash), "sha256:")
	if caHash == "" {
		caHash, err = caHashFromCertData(info.CAData)
		if err != nil {
			return nil, fmt.Errorf("计算 CA hash 失败（可手工指定 ca_cert_hash）: %w", err)
		}
	}

	token := strings.TrimSpace(options.Token)
	if token == "" {
		token, err = createBootstrapToken(ctx, info.Client)
		if err != nil {
			return nil, fmt.Errorf("创建 bootstrap token 失败（可手工指定 token）: %w", err)
		}
	}

	imageRepository := strings.TrimSpace(options.ImageRepository)
	if imageRepository == "" {
		imageRepository = info.Config.ImageRepository
	}

	return &joinConfig{
		APIServer:       apiServer,
		Token:           token,
		CAHash:          caHash,
		KubeRelease:     info.Config.KubernetesVersion,
		ImageRepository: imageRepository,
	}, nil
}

// normalizeAPIServer 统一为 host:port，并拒绝节点不可达的内部地址。
func normalizeAPIServer(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("无法确定 apiserver 地址，请手工指定 api_server")
	}

	host := raw
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", fmt.Errorf("解析 apiserver 地址失败: %w", err)
		}
		host = u.Host
	} else if strings.Contains(raw, "/") {
		host = strings.SplitN(raw, "/", 2)[0]
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return "", fmt.Errorf("apiserver 地址为空")
	}

	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	} else {
		host = net.JoinHostPort(host, "6443")
		hostname = strings.Trim(host, "[]")
		if h, _, err := net.SplitHostPort(host); err == nil {
			hostname = h
		}
	}

	lower := strings.ToLower(hostname)
	if lower == "localhost" || lower == "127.0.0.1" || lower == "::1" ||
		lower == "kubernetes.default.svc" || strings.HasSuffix(lower, ".svc") {
		return "", fmt.Errorf("apiserver 地址 %q 仅集群内可达，请手工指定 api_server（新节点可达的 VIP 或 master IP）", raw)
	}
	return host, nil
}

func caHashFromCertData(data []byte) (string, error) {
	rest := data
	for len(rest) > 0 {
		block, remaining := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = remaining
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
		return hex.EncodeToString(sum[:]), nil
	}
	return "", fmt.Errorf("kubeconfig 中缺少 CA 证书")
}

// createBootstrapToken 通过 Kubernetes API 创建 kubeadm bootstrap token。
func createBootstrapToken(ctx context.Context, cs *client.ClusterSet) (string, error) {
	if cs == nil || cs.Client == nil {
		return "", fmt.Errorf("集群客户端不可用")
	}
	tokenID, err := randomToken(6)
	if err != nil {
		return "", err
	}
	tokenSecret, err := randomToken(16)
	if err != nil {
		return "", err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bootstrap-token-" + tokenID,
			Namespace: "kube-system",
			Labels:    map[string]string{"pixiu-expand": "true"},
		},
		Type: corev1.SecretTypeBootstrapToken,
		StringData: map[string]string{
			"description":                    "pixiu cluster node expansion",
			"token-id":                       tokenID,
			"token-secret":                   tokenSecret,
			"usage-bootstrap-authentication": "true",
			"usage-bootstrap-signing":        "true",
			"auth-extra-groups":              "system:bootstrappers:kubeadm:default-node-token",
			"expiration":                     time.Now().Add(bootstrapTokenTTL).UTC().Format(time.RFC3339),
		},
	}
	if _, err = cs.Client.CoreV1().Secrets("kube-system").Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		return "", err
	}
	klog.Infof("created bootstrap token for cluster expansion: %s", secret.Name)
	return tokenID + "." + tokenSecret, nil
}

func randomToken(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	out := make([]byte, length)
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i := range out {
		out[i] = charset[int(buf[i])%len(charset)]
	}
	return string(out), nil
}

// clusterNodeAddresses 返回集群现有节点的 name 与 IP 集合，用于扩容前后查重/校正。
func clusterNodeAddresses(ctx context.Context, cs *client.ClusterSet) (map[string]struct{}, map[string]struct{}, error) {
	names := make(map[string]struct{})
	ips := make(map[string]struct{})
	if cs == nil || cs.Client == nil {
		return names, ips, fmt.Errorf("集群客户端不可用")
	}
	nodes, err := cs.Client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return names, ips, err
	}
	for i := range nodes.Items {
		node := nodes.Items[i]
		names[node.Name] = struct{}{}
		for _, addr := range node.Status.Addresses {
			if addr.Address != "" {
				ips[addr.Address] = struct{}{}
			}
		}
	}
	return names, ips, nil
}

// nodeMatched 节点是否已在集群中：IP 优先，name 兜底。
func nodeMatched(name, ip string, names, ips map[string]struct{}) bool {
	if ip != "" {
		if _, ok := ips[ip]; ok {
			return true
		}
	}
	_, ok := names[name]
	return ok
}

// resolveRunner 根据操作系统发行版解析 runner 镜像（与部署计划同一套解析规则）。
func (c *controller) resolveRunner(ctx context.Context, osImage string) (string, error) {
	osImage = strings.TrimSpace(osImage)
	if osImage == "" {
		return "", fmt.Errorf("请选择操作系统镜像（用于确定 runner）")
	}
	dist, err := c.factory.Distribution().GetDistributionByName(ctx, osImage)
	if err != nil {
		return "", err
	}
	if dist == nil || dist.Runner == "" {
		return "", fmt.Errorf("操作系统 %s 未配置 runner", osImage)
	}
	obj, err := c.factory.Runner().GetBy(ctx, db.WithName(dist.Runner))
	if err != nil {
		return "", err
	}
	if obj == nil || obj.EngineImage == "" {
		return "", fmt.Errorf("runner %s 不存在或未配置镜像", dist.Runner)
	}
	return obj.EngineImage, nil
}
