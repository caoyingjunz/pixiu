package cluster

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestCreateLegacyToken 验证老集群回退路径：复用已存在的 legacy SA token Secret。
func TestCreateLegacyToken(t *testing.T) {
	ctx := context.Background()
	ns := "pixiu-system"
	sa := "pixiu-sa-1"
	token := "legacy-token-abc"

	client := fake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sa + "-token-00001",
				Namespace: ns,
				Annotations: map[string]string{
					corev1.ServiceAccountNameKey: sa,
				},
			},
			Type: corev1.SecretTypeServiceAccountToken,
			Data: map[string][]byte{
				corev1.ServiceAccountTokenKey: []byte(token),
			},
		},
	)

	got, err := createLegacyToken(ctx, client, ns, sa)
	if err != nil {
		t.Fatalf("createLegacyToken() error = %v", err)
	}
	if got != token {
		t.Errorf("createLegacyToken() = %q, want %q", got, token)
	}
}
