package kind

import (
	"testing"
)

func TestKubeadmconfigPatch(t *testing.T) {
	expected := `{"kind":"ClusterConfiguration","etcd":{},"networking":{},"apiServer":{"extraArgs":{"api-audiences":"https://kubernetes.default.svc.cluster.local,sts.amazonaws.com","service-account-issuer":"https://somedomain","service-account-jwks-uri":"https://somedomain/keys.json"}},"controllerManager":{},"scheduler":{},"dns":{"type":""}}`

	if actual, err := toKubeadmConfigPatchYAML("somedomain"); err != nil {
		t.Error(err)
	} else if actual != expected {
		t.Errorf("expected: %s\n, got: %s\n", expected, actual)
	}
}
