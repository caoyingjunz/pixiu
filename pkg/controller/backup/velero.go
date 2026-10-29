package backup

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

const (
	storageLocationReadyTimeout      = 30 * time.Second
	storageLocationReadyPollInterval = 500 * time.Millisecond
)

var (
	backupGVR          = schema.GroupVersionResource{Group: "velero.io", Version: "v1", Resource: "backups"}
	scheduleGVR        = schema.GroupVersionResource{Group: "velero.io", Version: "v1", Resource: "schedules"}
	restoreGVR         = schema.GroupVersionResource{Group: "velero.io", Version: "v1", Resource: "restores"}
	storageLocationGVR = schema.GroupVersionResource{Group: "velero.io", Version: "v1", Resource: "backupstoragelocations"}
	secretGVR          = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	deploymentGVR      = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
)

type Client struct{ dynamic dynamic.Interface }

type StorageConfig struct {
	Provider              string
	Endpoint              string
	Bucket                string
	Prefix                string
	Region                string
	AccessKey             string
	SecretKey             string
	SessionToken          string
	UsePathStyle          bool
	InsecureSkipTLSVerify bool
}

type BackupSpec struct {
	Name                    string
	StorageLocation         string
	Namespaces              []string
	IncludedResources       []string
	LabelSelector           string
	IncludeClusterResources bool
	SnapshotVolumes         bool
	TTL                     string
}

type RestoreSpec struct {
	Name                 string
	BackupName           string
	IncludedNamespaces   []string
	ExcludedNamespaces   []string
	NamespaceMapping     map[string]string
	ExistingResourceMode string
	RestoreVolumes       bool
}

func NewVeleroClient(config *rest.Config) (*Client, error) {
	dc, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &Client{dynamic: dc}, nil
}

// AvailableNamespaces returns namespaces with a ready Velero deployment. It checks
// the API as well, so stale CRDs from an uninstall are not treated as available.
func (c *Client) AvailableNamespaces(ctx context.Context) ([]string, error) {
	_, err := c.dynamic.Resource(backupGVR).List(ctx, metav1.ListOptions{Limit: 1})
	if apierrors.IsNotFound(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	deployments, err := c.dynamic.Resource(deploymentGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	namespaces := make(map[string]struct{})
	for i := range deployments.Items {
		deployment := &deployments.Items[i]
		labels := deployment.GetLabels()
		if deployment.GetName() != "velero" && labels["app.kubernetes.io/name"] != "velero" && labels["component"] != "velero" {
			continue
		}
		available, _, _ := unstructured.NestedInt64(deployment.Object, "status", "availableReplicas")
		if available > 0 {
			namespaces[deployment.GetNamespace()] = struct{}{}
		}
	}
	result := make([]string, 0, len(namespaces))
	for namespace := range namespaces {
		result = append(result, namespace)
	}
	sort.Strings(result)
	return result, nil
}

func (c *Client) EnsureStorageLocation(ctx context.Context, namespace, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("Velero storage location is required")
	}
	_, err := c.dynamic.Resource(storageLocationGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Velero storage location %q is unavailable: %w", name, err)
	}
	return nil
}

// ConfigureStorageLocation 只负责创建/更新 BSL 及其凭据 Secret，不等待 BSL 变为 Available。
// 用于备份计划创建/编辑：桶内可能暂时存在非 Velero 目录等情况，此时不应阻塞计划保存；
// 完整性校验推迟到开始备份/恢复执行时（EnsureConfiguredStorageLocation）。
func (c *Client) ConfigureStorageLocation(ctx context.Context, namespace, name string, config *StorageConfig) error {
	if config == nil || strings.TrimSpace(config.Endpoint) == "" {
		return c.EnsureStorageLocation(ctx, namespace, name)
	}
	if strings.TrimSpace(config.Bucket) == "" || strings.TrimSpace(config.AccessKey) == "" || strings.TrimSpace(config.SecretKey) == "" {
		return fmt.Errorf("object storage bucket and credentials are required")
	}
	credentials := credentialsFile(config)
	secretName := storageSecretName(name)
	secretData := map[string]interface{}{"metadata": map[string]interface{}{"name": secretName}, "type": "Opaque", "stringData": map[string]interface{}{"cloud": credentials}}

	// Velero 在 BSL 更新后会重新校验对象存储，期间 status.phase 短暂变为 Unknown，
	// 此时创建 Backup 会被判为 Unavailable 而失败。因此只在内容真正变化时才更新；
	// 需要立即执行备份的场景由 EnsureConfiguredStorageLocation 负责等待 BSL 回到 Available。
	secretChanged := false
	secret := c.dynamic.Resource(secretGVR).Namespace(namespace)
	if existing, err := secret.Get(ctx, secretName, metav1.GetOptions{}); err == nil {
		if secretCredential(existing) != credentials {
			secretChanged = true
			secretObject := &unstructured.Unstructured{Object: secretData}
			secretObject.SetResourceVersion(existing.GetResourceVersion())
			if _, err = secret.Update(ctx, secretObject, metav1.UpdateOptions{}); err != nil {
				return fmt.Errorf("update Velero storage credential: %w", err)
			}
		}
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get Velero storage credential: %w", err)
	} else if _, err = secret.Create(ctx, &unstructured.Unstructured{Object: secretData}, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create Velero storage credential: %w", err)
	} else {
		secretChanged = true
	}

	bsl := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "velero.io/v1", "kind": "BackupStorageLocation",
		"metadata": map[string]interface{}{"name": name},
		"spec": map[string]interface{}{
			"provider":      providerName(config.Provider),
			"objectStorage": map[string]interface{}{"bucket": config.Bucket, "prefix": config.Prefix},
			"credential":    map[string]interface{}{"name": secretName, "key": "cloud"},
			"config":        map[string]interface{}{"region": config.Region, "s3Url": config.Endpoint, "s3ForcePathStyle": fmt.Sprintf("%t", config.UsePathStyle), "insecureSkipTLSVerify": fmt.Sprintf("%t", config.InsecureSkipTLSVerify)},
		},
	}}
	locations := c.dynamic.Resource(storageLocationGVR).Namespace(namespace)
	existing, err := locations.Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		// 无变化时不更新，避免无谓触发 Velero 重新校验；凭据变化时仍需更新 BSL 以触发校验。
		if !storageLocationMatches(existing, bsl) || secretChanged {
			bsl.SetResourceVersion(existing.GetResourceVersion())
			if _, err = locations.Update(ctx, bsl, metav1.UpdateOptions{}); err != nil {
				return fmt.Errorf("configure Velero storage location: %w", err)
			}
		}
	} else if apierrors.IsNotFound(err) {
		if _, err = locations.Create(ctx, bsl, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("configure Velero storage location: %w", err)
		}
	} else {
		return fmt.Errorf("get Velero storage location: %w", err)
	}
	return nil
}

// EnsureConfiguredStorageLocation 在 ConfigureStorageLocation 基础上等待 BSL 变为 Available，
// 用于开始备份/恢复等必须确认对象存储可用的场景。
func (c *Client) EnsureConfiguredStorageLocation(ctx context.Context, namespace, name string, config *StorageConfig) error {
	if config == nil || strings.TrimSpace(config.Endpoint) == "" {
		return c.EnsureStorageLocation(ctx, namespace, name)
	}
	if err := c.ConfigureStorageLocation(ctx, namespace, name, config); err != nil {
		return err
	}
	// 无论本次是否更新，都要确认 BSL 已就绪：上一次可能遗留 Unknown/Unavailable。
	return waitStorageLocationAvailable(ctx, c.dynamic.Resource(storageLocationGVR).Namespace(namespace), name)
}

// waitStorageLocationAvailable 轮询 BSL 状态，直到 Available 或超时；
// Unavailable 立即返回携带 Velero message 的错误。
func waitStorageLocationAvailable(ctx context.Context, locations dynamic.ResourceInterface, name string) error {
	var lastErr error
	err := wait.PollUntilContextTimeout(ctx, storageLocationReadyPollInterval, storageLocationReadyTimeout, true,
		func(ctx context.Context) (bool, error) {
			current, err := locations.Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				lastErr = err
				return false, nil
			}
			phase, _, _ := unstructured.NestedString(current.Object, "status", "phase")
			switch strings.ToLower(strings.TrimSpace(phase)) {
			case "available":
				return true, nil
			case "unavailable":
				message, _, _ := unstructured.NestedString(current.Object, "status", "message")
				if strings.TrimSpace(message) == "" {
					message = "Velero reports that the object storage is unavailable"
				}
				return false, fmt.Errorf("object storage connection failed: %s", message)
			default:
				return false, nil
			}
		})
	if err == nil {
		return nil
	}
	if wait.Interrupted(err) {
		if lastErr != nil {
			return fmt.Errorf("wait for Velero storage location %q: %w", name, lastErr)
		}
		return fmt.Errorf("wait for Velero storage location %q to become available: timed out after %s", name, storageLocationReadyTimeout)
	}
	return err
}

// secretCredential 读取 Secret 中的 cloud 凭据内容，兼容 base64 字符串与 []byte 两种表示。
func secretCredential(secret *unstructured.Unstructured) string {
	data, _, _ := unstructured.NestedMap(secret.Object, "data")
	switch value := data["cloud"].(type) {
	case string:
		decoded, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return ""
		}
		return string(decoded)
	case []byte:
		return string(value)
	default:
		return ""
	}
}

// storageLocationMatches 只比较我们下发的字段，避免 CRD 默认值造成误判。
func storageLocationMatches(existing, desired *unstructured.Unstructured) bool {
	for _, path := range [][]string{
		{"spec", "provider"},
		{"spec", "objectStorage", "bucket"},
		{"spec", "objectStorage", "prefix"},
		{"spec", "credential", "name"},
		{"spec", "credential", "key"},
	} {
		want, _, _ := unstructured.NestedString(desired.Object, path...)
		got, _, _ := unstructured.NestedString(existing.Object, path...)
		if want != got {
			return false
		}
	}
	wantConfig, _, _ := unstructured.NestedMap(desired.Object, "spec", "config")
	gotConfig, _, _ := unstructured.NestedMap(existing.Object, "spec", "config")
	return apiequality.Semantic.DeepEqual(wantConfig, gotConfig)
}

func credentialsFile(config *StorageConfig) string {
	content := fmt.Sprintf("[default]\naws_access_key_id=%s\naws_secret_access_key=%s\n", config.AccessKey, config.SecretKey)
	if config.SessionToken != "" {
		content += fmt.Sprintf("aws_session_token=%s\n", config.SessionToken)
	}
	return content
}

func storageSecretName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, name)
	return strings.Trim(name, "-") + "-credentials"
}

// providerName Velero 的 BackupStorageLocation 统一使用 S3 兼容协议访问对象存储
// （AWS S3、MinIO、阿里云 OSS、腾讯云 COS、华为云 OBS 等均兼容），provider 固定为 aws；
// 各存储的差异（endpoint、path-style 等）由 StorageConfig 单独下发。
func providerName(string) string { return "aws" }

func (c *Client) CreateBackup(ctx context.Context, namespace string, spec BackupSpec) (*unstructured.Unstructured, error) {
	object := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "velero.io/v1", "kind": "Backup",
		"metadata": map[string]interface{}{"name": spec.Name},
		"spec": map[string]interface{}{
			"storageLocation":         spec.StorageLocation,
			"includedNamespaces":      stringSlice(spec.Namespaces),
			"includedResources":       stringSlice(spec.IncludedResources),
			"includeClusterResources": spec.IncludeClusterResources,
			"snapshotVolumes":         spec.SnapshotVolumes,
			"ttl":                     spec.TTL,
		},
	}}
	if spec.LabelSelector != "" {
		selector, err := labelSelector(spec.LabelSelector)
		if err != nil {
			return nil, err
		}
		if err = unstructured.SetNestedMap(object.Object, selector, "spec", "labelSelector"); err != nil {
			return nil, err
		}
	}
	return c.dynamic.Resource(backupGVR).Namespace(namespace).Create(ctx, object, metav1.CreateOptions{})
}

func (c *Client) GetBackup(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	return c.dynamic.Resource(backupGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// DeleteBackup 删除 Velero Backup；对象不存在视为已删除（幂等）。
func (c *Client) DeleteBackup(ctx context.Context, namespace, name string) error {
	err := c.dynamic.Resource(backupGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *Client) CreateSchedule(ctx context.Context, namespace, name, cron string, spec BackupSpec) (*unstructured.Unstructured, error) {
	object := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "velero.io/v1", "kind": "Schedule",
		"metadata": map[string]interface{}{"name": name},
		"spec": map[string]interface{}{
			"schedule": cron,
			"template": map[string]interface{}{
				"storageLocation":         spec.StorageLocation,
				"includedNamespaces":      stringSlice(spec.Namespaces),
				"includedResources":       stringSlice(spec.IncludedResources),
				"includeClusterResources": spec.IncludeClusterResources,
				"snapshotVolumes":         spec.SnapshotVolumes,
				"ttl":                     spec.TTL,
			},
		},
	}}
	if spec.LabelSelector != "" {
		selector, err := labelSelector(spec.LabelSelector)
		if err != nil {
			return nil, err
		}
		if err = unstructured.SetNestedMap(object.Object, selector, "spec", "template", "labelSelector"); err != nil {
			return nil, err
		}
	}
	return c.dynamic.Resource(scheduleGVR).Namespace(namespace).Create(ctx, object, metav1.CreateOptions{})
}

func (c *Client) UpsertSchedule(ctx context.Context, namespace, name, cron string, spec BackupSpec) (*unstructured.Unstructured, error) {
	object, err := c.CreateSchedule(ctx, namespace, name, cron, spec)
	if err == nil {
		return object, nil
	}
	existing, getErr := c.dynamic.Resource(scheduleGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if getErr != nil {
		return nil, err
	}
	if setErr := unstructured.SetNestedField(existing.Object, cron, "spec", "schedule"); setErr != nil {
		return nil, setErr
	}
	template, _, _ := unstructured.NestedMap(existing.Object, "spec", "template")
	template["storageLocation"] = spec.StorageLocation
	template["includedNamespaces"] = stringSlice(spec.Namespaces)
	template["includedResources"] = stringSlice(spec.IncludedResources)
	template["includeClusterResources"] = spec.IncludeClusterResources
	template["snapshotVolumes"] = spec.SnapshotVolumes
	template["ttl"] = spec.TTL
	if spec.LabelSelector == "" {
		delete(template, "labelSelector")
	} else {
		selector, selectorErr := labelSelector(spec.LabelSelector)
		if selectorErr != nil {
			return nil, selectorErr
		}
		template["labelSelector"] = selector
	}
	if setErr := unstructured.SetNestedMap(existing.Object, template, "spec", "template"); setErr != nil {
		return nil, setErr
	}
	return c.dynamic.Resource(scheduleGVR).Namespace(namespace).Update(ctx, existing, metav1.UpdateOptions{})
}

func (c *Client) DeleteSchedule(ctx context.Context, namespace, name string) error {
	return c.dynamic.Resource(scheduleGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func (c *Client) CreateRestore(ctx context.Context, namespace string, spec RestoreSpec) (*unstructured.Unstructured, error) {
	restoreSpec := map[string]interface{}{
		"backupName":             spec.BackupName,
		"existingResourcePolicy": spec.ExistingResourceMode,
		"restorePVs":             spec.RestoreVolumes,
	}
	if len(spec.IncludedNamespaces) > 0 {
		restoreSpec["includedNamespaces"] = stringSlice(spec.IncludedNamespaces)
	}
	if len(spec.ExcludedNamespaces) > 0 {
		restoreSpec["excludedNamespaces"] = stringSlice(spec.ExcludedNamespaces)
	}
	object := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "velero.io/v1", "kind": "Restore",
		"metadata": map[string]interface{}{"name": spec.Name},
		"spec":     restoreSpec,
	}}
	if len(spec.NamespaceMapping) > 0 {
		_ = unstructured.SetNestedStringMap(object.Object, spec.NamespaceMapping, "spec", "namespaceMapping")
	}
	return c.dynamic.Resource(restoreGVR).Namespace(namespace).Create(ctx, object, metav1.CreateOptions{})
}

func (c *Client) GetRestore(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	return c.dynamic.Resource(restoreGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (c *Client) DeleteRestore(ctx context.Context, namespace, name string) error {
	return c.dynamic.Resource(restoreGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func stringSlice(values []string) []interface{} {
	result := make([]interface{}, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	return result
}

func labelSelector(value string) (map[string]interface{}, error) {
	selector, err := metav1.ParseToLabelSelector(value)
	if err != nil {
		return nil, fmt.Errorf("invalid label selector %q: %w", value, err)
	}
	raw, err := json.Marshal(selector)
	if err != nil {
		return nil, err
	}
	result := map[string]interface{}{}
	if err = json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, nil
}
