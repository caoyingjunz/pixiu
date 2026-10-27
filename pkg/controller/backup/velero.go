package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
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

func (c *Client) EnsureConfiguredStorageLocation(ctx context.Context, namespace, name string, config *StorageConfig) error {
	if config == nil || strings.TrimSpace(config.Endpoint) == "" {
		return c.EnsureStorageLocation(ctx, namespace, name)
	}
	if strings.TrimSpace(config.Bucket) == "" || strings.TrimSpace(config.AccessKey) == "" || strings.TrimSpace(config.SecretKey) == "" {
		return fmt.Errorf("object storage bucket and credentials are required")
	}
	secretName := storageSecretName(name)
	secretData := map[string]interface{}{"metadata": map[string]interface{}{"name": secretName}, "type": "Opaque", "stringData": map[string]interface{}{"cloud": credentialsFile(config)}}
	secret := c.dynamic.Resource(secretGVR).Namespace(namespace)
	if existing, err := secret.Get(ctx, secretName, metav1.GetOptions{}); err == nil {
		secretObject := &unstructured.Unstructured{Object: secretData}
		secretObject.SetResourceVersion(existing.GetResourceVersion())
		if _, err = secret.Update(ctx, secretObject, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("update Velero storage credential: %w", err)
		}
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get Velero storage credential: %w", err)
	} else if _, err = secret.Create(ctx, &unstructured.Unstructured{Object: secretData}, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create Velero storage credential: %w", err)
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
		bsl.SetResourceVersion(existing.GetResourceVersion())
		_, err = locations.Update(ctx, bsl, metav1.UpdateOptions{})
	} else if apierrors.IsNotFound(err) {
		_, err = locations.Create(ctx, bsl, metav1.CreateOptions{})
	} else {
		return fmt.Errorf("get Velero storage location: %w", err)
	}
	if err != nil {
		return fmt.Errorf("configure Velero storage location: %w", err)
	}
	if current, getErr := locations.Get(ctx, name, metav1.GetOptions{}); getErr == nil {
		phase, _, _ := unstructured.NestedString(current.Object, "status", "phase")
		message, _, _ := unstructured.NestedString(current.Object, "status", "message")
		if strings.EqualFold(phase, "Unavailable") {
			if message == "" {
				message = "Velero reports that the object storage is unavailable"
			}
			return fmt.Errorf("object storage connection failed: %s", message)
		}
	}
	return nil
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

func (c *Client) DeleteBackup(ctx context.Context, namespace, name string) error {
	return c.dynamic.Resource(backupGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
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
