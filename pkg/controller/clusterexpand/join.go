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
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"

	"github.com/caoyingjunz/pixiu/api/server/errors"
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
	Capabilities *types.ClusterExpansionCapabilities
	Client       *client.ClusterSet
	Kubeadm      *kubeadmClusterConfiguration
	Server       string
	CAData       []byte
}

type joinConfig struct {
	APIServer       string
	Token           string
	CAHash          string
	KubeRelease     string
	ImageRepository string
}

// detectClusterInfo 探测集群是否具备扩容能力，并返回 join 所需的原始信息。
func (c *controller) detectClusterInfo(ctx context.Context, obj *model.Cluster) *clusterInfo {
	info := &clusterInfo{
		Capabilities: &types.ClusterExpansionCapabilities{},
	}

	opts := clustercontroller.ResolveClusterSetOptions(ctx, c.factory, obj)
	cs, err := client.NewClusterSetWithOptions(obj.KubeConfig, opts)
	if err != nil {
		info.Capabilities.Reason = fmt.Sprintf("加载集群 kubeconfig 失败: %v", err)
		return info
	}
	info.Client = cs

	// 1) kubeadm 体系判定 add-node 本质是 kubeadm join
	cm, err := cs.Client.CoreV1().ConfigMaps("kube-system").Get(ctx, kubeadmConfigMapName, metav1.GetOptions{})
	if err != nil {
		info.Capabilities.Reason = "未检测到 kube-system/kubeadm-config，当前集群不是 kubeadm 体系，暂不支持扩容"
		return info
	}
	var kubeadmCfg kubeadmClusterConfiguration
	if err = yaml.Unmarshal([]byte(cm.Data[kubeadmConfigDataKey]), &kubeadmCfg); err != nil {
		info.Capabilities.Reason = fmt.Sprintf("解析 kubeadm-config 失败: %v", err)
		return info
	}
	info.Kubeadm = &kubeadmCfg

	// 2) kubeconfig 中的 apiserver 与 CA
	rawKubeconfig, err := client.ParseKubeConfigBytes(obj.KubeConfig)
	if err != nil {
		info.Capabilities.Reason = fmt.Sprintf("解析集群 kubeconfig 失败: %v", err)
		return info
	}
	kubeconfig, err := clientcmd.Load(rawKubeconfig)
	if err != nil {
		info.Capabilities.Reason = fmt.Sprintf("加载集群 kubeconfig 失败: %v", err)
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
		info.Capabilities.Reason = fmt.Sprintf("查询集群节点失败: %v", err)
		return info
	}
	if len(nodes.Items) == 0 {
		info.Capabilities.Reason = "集群暂无可探测的节点，无法确认容器运行时"
		return info
	}
	// 全节点必须是 containerd，docker、cri-o 以及混合运行时均不支持
	for i := range nodes.Items {
		runtimeVersion := nodes.Items[i].Status.NodeInfo.ContainerRuntimeVersion
		if !strings.HasPrefix(runtimeVersion, "containerd://") {
			info.Capabilities.Reason = "当前集群容器运行时不是纯 containerd，暂不支持扩容（kubez-ansible add-node 仅支持 containerd）"
			return info
		}
	}

	// 4) 版本 / 镜像仓库 / 控制面地址
	version := strings.TrimPrefix(strings.TrimSpace(kubeadmCfg.KubernetesVersion), "v")
	if version == "" {
		if sv, verr := cs.Client.Discovery().ServerVersion(); verr == nil {
			version = strings.TrimPrefix(sv.GitVersion, "v")
		}
	}
	info.Capabilities.KubernetesVersion = version
	info.Capabilities.ImageRepository = strings.TrimSpace(kubeadmCfg.ImageRepository)
	info.Capabilities.ControlPlaneEndpoint = strings.TrimSpace(kubeadmCfg.ControlPlaneEndpoint)

	// 5) 计划存在时仅作为默认值（不参与执行依赖）
	info.Capabilities.DefaultNetworkInterface = "eth0"
	if obj.PlanId > 0 {
		if cfg, cerr := c.factory.Plan().Config().GetByPlan(ctx, obj.PlanId); cerr == nil && cfg != nil {
			info.Capabilities.DefaultOSImage = cfg.OSImage
			networkSpec := types.NetworkSpec{}
			if networkSpec.Unmarshal(cfg.Network) == nil && networkSpec.NetworkInterface != "" {
				info.Capabilities.DefaultNetworkInterface = networkSpec.NetworkInterface
			}
		}
	}

	info.Capabilities.Supported = true
	return info
}

// expansionPlanJoinParams 读取集群扩容计划（plans.type=1）配置中持久化的 join 参数。
// 无计划 / 无配置 / 解析失败时返回空值，不报错。
func (c *controller) expansionPlanJoinParams(ctx context.Context, clusterId int64) (bootstrapToken, caHash, apiServer string) {
	plans, err := c.factory.Plan().List(ctx, db.WithPlanTypeEq(1), db.WithPlanClusterIdEq(clusterId))
	if err != nil || len(plans) == 0 {
		return "", "", ""
	}
	cfg, err := c.factory.Plan().Config().GetByPlan(ctx, plans[0].Id)
	if err != nil || cfg == nil {
		return "", "", ""
	}
	ks := types.KubernetesSpec{}
	if err = ks.Unmarshal(cfg.Kubernetes); err != nil {
		return "", "", ""
	}
	return strings.TrimSpace(ks.BootstrapToken), strings.TrimSpace(ks.CAHash), strings.TrimSpace(ks.KubeAPIServerAddress)
}

// resolveJoin 计算 join 所需的参数：优先请求覆盖，其次扩容计划配置，再次 kubeadm-config，最后 kubeconfig。
func (c *controller) resolveJoin(ctx context.Context, obj *model.Cluster, info *clusterInfo, options *types.ExpansionOptions) (*joinConfig, error) {
	if info == nil || info.Capabilities == nil || !info.Capabilities.Supported {
		reason := "集群不支持扩容"
		if info != nil && info.Capabilities != nil && info.Capabilities.Reason != "" {
			reason = info.Capabilities.Reason
		}
		return nil, fmt.Errorf("%s", reason)
	}

	planToken, planCAHash, planAPIServer := c.expansionPlanJoinParams(ctx, obj.Id)

	apiServer := strings.TrimSpace(options.APIServer)
	if apiServer == "" {
		apiServer = planAPIServer
	}
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
		caHash = strings.TrimPrefix(planCAHash, "sha256:")
	}
	if caHash == "" {
		caHash, err = caHashFromCertData(info.CAData)
		if err != nil {
			return nil, fmt.Errorf("计算 CA hash 失败（可手工指定 ca_cert_hash）: %w", err)
		}
	}

	token := strings.TrimSpace(options.Token)
	if token == "" {
		token = planToken
	}
	if token == "" {
		token, err = createBootstrapToken(ctx, info.Client)
		if err != nil {
			return nil, fmt.Errorf("创建 bootstrap token 失败（可手工指定 token）: %w", err)
		}
	}

	imageRepository := strings.TrimSpace(options.ImageRepository)
	if imageRepository == "" {
		imageRepository = info.Capabilities.ImageRepository
	}

	return &joinConfig{
		APIServer:       apiServer,
		Token:           token,
		CAHash:          caHash,
		KubeRelease:     info.Capabilities.KubernetesVersion,
		ImageRepository: imageRepository,
	}, nil
}

// GenerateJoinParams 通过 Kubernetes API 自动生成一次性 join 参数，供扩容页面「自动生成」按钮填充。
// token 由 K8s API 创建（bootstrap-token Secret，TTL 2 小时），CA hash 由 kubeconfig 中的 CA 证书计算，
// apiserver 取 kubeadm-config 的 controlPlaneEndpoint，缺省回落 kubeconfig 的 server。
func (c *controller) GenerateJoinParams(ctx context.Context, clusterId int64) (*types.ExpansionJoinParams, error) {
	obj, _, err := c.authorize(ctx, clusterId)
	if err != nil {
		return nil, err
	}

	info := c.detectClusterInfo(ctx, obj)
	if info == nil || info.Capabilities == nil || !info.Capabilities.Supported {
		reason := "集群不支持扩容"
		if info != nil && info.Capabilities != nil && info.Capabilities.Reason != "" {
			reason = info.Capabilities.Reason
		}
		return nil, errors.NewError(fmt.Errorf("%s", reason), http.StatusBadRequest)
	}

	apiServer := strings.TrimSpace(info.Capabilities.ControlPlaneEndpoint)
	if apiServer == "" {
		apiServer = info.Server
	}
	apiServer, err = normalizeAPIServer(apiServer)
	if err != nil {
		return nil, errors.NewError(err, http.StatusBadRequest)
	}

	caHash, err := caHashFromCertData(info.CAData)
	if err != nil {
		return nil, errors.NewError(fmt.Errorf("计算 CA hash 失败（可在 kubeconfig 中补充 CA 证书后重试）: %w", err), http.StatusBadGateway)
	}

	token, err := createBootstrapToken(ctx, info.Client)
	if err != nil {
		return nil, errors.NewError(fmt.Errorf("创建 bootstrap token 失败（请确认 kubeconfig 具备 kube-system secrets 创建权限）: %w", err), http.StatusBadGateway)
	}

	return &types.ExpansionJoinParams{
		APIServer: apiServer,
		Token:     token,
		CAHash:    caHash,
		ExpiresAt: time.Now().Add(bootstrapTokenTTL),
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
	// 用 rand.Int 按字符集大小取随机数，避免 byte % len(charset) 的取模偏差
	limit := big.NewInt(int64(len(charset)))
	out := make([]byte, length)
	for i := range out {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		out[i] = charset[n.Int64()]
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
