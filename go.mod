module github.com/frezbo/irsa-anywhere

go 1.16

require (
	github.com/frezbo/pulumi-provider-kind/sdk/v3 v3.0.0-20210731173108-242c2de77c57
	github.com/pkg/errors v0.9.1
	github.com/pulumi/pulumi-aws/sdk/v4 v4.24.0
	github.com/pulumi/pulumi-kubernetes/sdk/v3 v3.8.0
	github.com/pulumi/pulumi-tls/sdk/v4 v4.0.0
	github.com/pulumi/pulumi/sdk/v3 v3.14.1-0.20211007222624-789e39219452
	k8s.io/apimachinery v0.22.2
	k8s.io/kubernetes v1.21.3
	sigs.k8s.io/kind v0.11.1
)

replace k8s.io/api => k8s.io/api v0.21.2

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.21.2

replace k8s.io/apimachinery => k8s.io/apimachinery v0.21.3-rc.0

replace k8s.io/apiserver => k8s.io/apiserver v0.21.2

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.21.2

replace k8s.io/client-go => k8s.io/client-go v0.21.2

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.21.2

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.21.2

replace k8s.io/code-generator => k8s.io/code-generator v0.21.3-rc.0

replace k8s.io/component-base => k8s.io/component-base v0.21.2

replace k8s.io/component-helpers => k8s.io/component-helpers v0.21.2

replace k8s.io/controller-manager => k8s.io/controller-manager v0.21.2

replace k8s.io/cri-api => k8s.io/cri-api v0.21.3-rc.0

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.21.2

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.21.2

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.21.2

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.21.2

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.21.2

replace k8s.io/kubectl => k8s.io/kubectl v0.21.2

replace k8s.io/kubelet => k8s.io/kubelet v0.21.2

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.21.2

replace k8s.io/metrics => k8s.io/metrics v0.21.2

replace k8s.io/mount-utils => k8s.io/mount-utils v0.21.3-rc.0

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.21.2

replace k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.21.2

replace k8s.io/sample-controller => k8s.io/sample-controller v0.21.2
