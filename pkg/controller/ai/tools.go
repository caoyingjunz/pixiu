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

package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/client"
	clustercontroller "github.com/caoyingjunz/pixiu/pkg/controller/cluster"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"
)

type toolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
	Handler     func(ctx context.Context, args map[string]interface{}) (string, error)
}

func (c *controller) buildTools() []toolDefinition {
	return []toolDefinition{
		{
			Name:        "k8s",
			Description: "Operate on Kubernetes resources in a Pixiu cluster via preinstalled kubectl and in-memory cluster config.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"cluster": map[string]interface{}{
						"type":        "string",
						"description": "Pixiu cluster name.",
					},
					"args": map[string]interface{}{
						"type":        "array",
						"description": "kubectl arguments excluding kubeconfig, for example ['get','pods','-A','-o','json'] or ['logs','pod-name','-n','default'].",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"required":             []string{"cluster", "args"},
				"additionalProperties": false,
			},
			Handler: c.handleK8s,
		},
	}
}

func toResponsesTools(tools []toolDefinition) []map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(tools))
	for _, tool := range tools {
		items = append(items, map[string]interface{}{
			"type":        "function",
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  tool.Parameters,
		})
	}
	return items
}

func (c *controller) executeTool(ctx context.Context, callID, toolName, rawArgs string) (string, error) {
	var args map[string]interface{}
	if rawArgs != "" {
		if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
			return "", fmt.Errorf("invalid tool arguments: %v", err)
		}
	}

	start := time.Now()
	var (
		output string
		err    error
	)
	for _, tool := range c.buildTools() {
		if tool.Name == toolName {
			output, err = tool.Handler(ctx, args)
			c.recordToolExecution(ctx, callID, toolName, rawArgs, output, err, time.Since(start))
			return output, err
		}
	}
	err = fmt.Errorf("unknown tool: %s", toolName)
	c.recordToolExecution(ctx, callID, toolName, rawArgs, "", err, time.Since(start))
	return "", err
}

func (c *controller) handleK8s(ctx context.Context, args map[string]interface{}) (string, error) {
	clusterName, _ := args["cluster"].(string)
	clusterName = strings.TrimSpace(clusterName)
	if clusterName == "" {
		return "", fmt.Errorf("cluster is required")
	}

	kubectlArgs, err := parseStringArgs(args["args"])
	if err != nil {
		return "", err
	}
	if len(kubectlArgs) == 0 {
		return "", fmt.Errorf("args is required")
	}

	body, err := c.runKubectlWithCluster(ctx, clusterName, kubectlArgs)
	if err != nil {
		return "", err
	}
	return wrapK8SResult(clusterName, kubectlArgs, string(body))
}

func (c *controller) runKubectlWithCluster(ctx context.Context, clusterName string, args []string) ([]byte, error) {
	clusterSet, err := c.getClusterSetForAI(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	kubeconfigBytes, err := buildKubeconfigFromRESTConfig(clusterName, clusterSet.Config)
	if err != nil {
		return nil, err
	}

	tempFile, err := os.CreateTemp("", "pixiu-kubeconfig-*.yaml")
	if err != nil {
		return nil, err
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err = tempFile.Write(kubeconfigBytes); err != nil {
		_ = tempFile.Close()
		return nil, err
	}
	if err = tempFile.Close(); err != nil {
		return nil, err
	}

	cmdArgs := append([]string{"--kubeconfig", tempPath}, args...)
	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	kubectlPath, err := findKubectlBinary()
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(cmdCtx, kubectlPath, cmdArgs...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err = cmd.Run(); err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText == "" {
			errText = err.Error()
		}
		return nil, fmt.Errorf("kubectl failed: %s", truncateToolOutput(errText))
	}
	return stdout.Bytes(), nil
}

func (c *controller) getClusterSetForAI(ctx context.Context, clusterName string) (client.ClusterSet, error) {
	clusterSet, ok := clustercontroller.ClusterIndexer.Get(clusterName)
	if ok {
		if clusterSet.Config == nil {
			return client.ClusterSet{}, fmt.Errorf("cluster %q has empty in-memory config", clusterName)
		}
		return clusterSet, nil
	}

	object, err := c.factory.Cluster().GetClusterByName(ctx, clusterName)
	if err != nil {
		return client.ClusterSet{}, err
	}
	if object == nil {
		return client.ClusterSet{}, fmt.Errorf("cluster %q not found", clusterName)
	}
	if strings.TrimSpace(object.KubeConfig) == "" {
		return client.ClusterSet{}, fmt.Errorf("cluster %q kubeconfig is empty", clusterName)
	}

	newClusterSet, err := client.NewClusterSet(object.KubeConfig)
	if err != nil {
		return client.ClusterSet{}, fmt.Errorf("build cluster config for %q failed: %v", clusterName, err)
	}

	clustercontroller.ClusterIndexer.Set(clusterName, *newClusterSet)
	return *newClusterSet, nil
}

func findKubectlBinary() (string, error) {
	if path, err := exec.LookPath("kubectl"); err == nil {
		return path, nil
	}

	localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	candidates := []string{
		filepath.Join(localAppData, "Microsoft", "WinGet", "Packages", "Kubernetes.kubectl_Microsoft.Winget.Source_8wekyb3d8bbwe", "kubectl.exe"),
		`C:\Program Files\Kubernetes\kubectl.exe`,
		`C:\kubectl\kubectl.exe`,
	}

	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("kubectl not found in PATH or known install locations")
}

func buildKubeconfigFromRESTConfig(clusterName string, cfg *restclient.Config) ([]byte, error) {
	if cfg == nil {
		return nil, fmt.Errorf("rest config is nil")
	}

	clusterConfig := &clientcmdapi.Cluster{
		Server:                   cfg.Host,
		CertificateAuthorityData: cfg.TLSClientConfig.CAData,
		InsecureSkipTLSVerify:    cfg.TLSClientConfig.Insecure,
		TLSServerName:            cfg.TLSClientConfig.ServerName,
	}
	authInfo := &clientcmdapi.AuthInfo{
		Token:                 cfg.BearerToken,
		Username:              cfg.Username,
		Password:              cfg.Password,
		ClientCertificateData: cfg.TLSClientConfig.CertData,
		ClientKeyData:         cfg.TLSClientConfig.KeyData,
	}

	kubeconfig := clientcmdapi.Config{
		Kind:           "Config",
		APIVersion:     "v1",
		CurrentContext: clusterName,
		Clusters: map[string]*clientcmdapi.Cluster{
			clusterName: clusterConfig,
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			clusterName: authInfo,
		},
		Contexts: map[string]*clientcmdapi.Context{
			clusterName: {
				Cluster:  clusterName,
				AuthInfo: clusterName,
			},
		},
	}

	data, err := clientcmd.Write(kubeconfig)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func wrapK8SResult(clusterName string, kubectlArgs []string, body string) (string, error) {
	result := map[string]interface{}{
		"cluster": clusterName,
		"args":    kubectlArgs,
		"success": true,
		"body":    truncateToolOutput(body),
	}
	data, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func parseStringArgs(value interface{}) ([]string, error) {
	if value == nil {
		return nil, nil
	}

	rawItems, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf("args must be a string array")
	}

	items := make([]string, 0, len(rawItems))
	for _, item := range rawItems {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("args must be a string array")
		}
		items = append(items, text)
	}
	return items, nil
}

func truncateToolOutput(text string) string {
	text = strings.TrimSpace(text)
	if len(text) <= 4000 {
		return text
	}
	return text[:4000]
}

func (c *controller) recordToolExecution(ctx context.Context, callID, toolName, rawArgs, output string, runErr error, duration time.Duration) {
	meta := getToolExecutionMeta(ctx)
	if meta == nil {
		return
	}

	recordCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	record := &model.Execution{
		RequestId:      meta.RequestId,
		UserId:         meta.UserId,
		UserName:       meta.UserName,
		ProviderId:     meta.ProviderId,
		ConversationId: meta.ConversationId,
		Provider:       meta.Provider,
		ModelName:      meta.ModelName,
		ToolName:       toolName,
		CallId:         callID,
		Arguments:      truncateToolOutput(rawArgs),
		Output:         truncateToolOutput(output),
		Success:        runErr == nil,
		Duration:       duration.Milliseconds(),
	}
	if runErr != nil {
		record.ErrorMessage = truncateToolOutput(runErr.Error())
	}

	if _, err := c.factory.AI().Execution().Create(recordCtx, record); err != nil {
		klog.Errorf("failed to create ai execution record for tool(%s): %v", toolName, err)
	}
}
