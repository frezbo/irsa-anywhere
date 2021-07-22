package kind

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/frezbo/irsa-anywhere/pkg/apps/irsa"
	"github.com/frezbo/irsa-anywhere/pkg/apps/sampleapp"
	awsmeta "github.com/frezbo/irsa-anywhere/pkg/aws/meta"
	"github.com/frezbo/irsa-anywhere/pkg/component"
	"github.com/frezbo/irsa-anywhere/pkg/oidc"
	"github.com/frezbo/irsa-anywhere/pkg/resource"
	"github.com/frezbo/pulumi-provider-kind/sdk/v3/go/kind/cluster"
	"github.com/frezbo/pulumi-provider-kind/sdk/v3/go/kind/networking"
	"github.com/frezbo/pulumi-provider-kind/sdk/v3/go/kind/node"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/s3"
	"github.com/pulumi/pulumi-tls/sdk/v4/go/tls"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumiconfig "github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeadmv1beta2 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta2"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	kindcluster "sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
)

func NewKindConfig(ctx *pulumi.Context, name string) resource.Resource {
	return &kindConfig{
		pulumiContext: ctx,
		name:          name,
	}
}

func (c *kindConfig) Create() (pulumi.Resource, error) {
	kindResource, err := component.NewDynamicComponent(c.pulumiContext, c.name)
	if err != nil {
		return nil, err
	}

	commonAwsResourceTags, err := awsmeta.ResourceTags(c.pulumiContext, c.name)
	if err != nil {
		return nil, err
	}

	bucket, err := s3.NewBucket(c.pulumiContext, c.name, &s3.BucketArgs{
		Tags: commonAwsResourceTags,
	}, pulumi.Parent(kindResource))
	if err != nil {
		return nil, err
	}

	oidcData := bucket.BucketRegionalDomainName.ApplyT(func(domain string) (map[string]string, error) {
		oidcData := map[string]string{
			"domain": domain,
		}

		kubeadmConfigPatch, err := toKubeadmConfigPatchYAML(domain)
		if err != nil {
			return oidcData, err
		}
		oidcData["kubeadmConfigPatch"] = kubeadmConfigPatch

		certs, err := tls.GetCertificate(c.pulumiContext, &tls.GetCertificateArgs{
			Url:         fmt.Sprintf("https://%s", domain),
			VerifyChain: &[]bool{true}[0],
		})
		if err != nil {
			return oidcData, err
		}

		for _, cert := range certs.Certificates {
			if cert.IsCa {
				oidcData["caFingerprint"] = cert.Sha1Fingerprint
				break
			}
		}

		return oidcData, nil
	}).(pulumi.StringMapOutput)

	openIDProvider, err := iam.NewOpenIdConnectProvider(c.pulumiContext, c.name, &iam.OpenIdConnectProviderArgs{
		Url:             pulumi.Sprintf("https://%s", oidcData.MapIndex(pulumi.String("domain"))),
		ClientIdLists:   pulumi.StringArray{pulumi.String("sts.amazonaws.com")},
		ThumbprintLists: pulumi.StringArray{oidcData.MapIndex(pulumi.String("caFingerprint"))},
		Tags:            commonAwsResourceTags,
	}, pulumi.Parent(kindResource))
	if err != nil {
		return nil, err
	}

	cluster, err := cluster.NewCluster(c.pulumiContext, c.name, &cluster.ClusterArgs{
		// TODO: remove, added for testing
		Networking: networking.NetworkingArgs{
			ApiServerAddress: pulumi.String("0.0.0.0"),
		},
		Nodes: node.NodeArray{
			node.NodeArgs{
				Role: node.RoleTypeControlPlane,
				KubeadmConfigPatches: pulumi.StringArray{
					oidcData.MapIndex(pulumi.String("kubeadmConfigPatch")),
				},
			},
		},
	}, pulumi.Parent(kindResource))
	if err != nil {
		return nil, err
	}

	oidcConfig := cluster.Name.ApplyT(func(name string) (map[string]string, error) {
		c.pulumiContext.Log.Info("getting oidc config from cluster...", &pulumi.LogArgs{
			Resource: cluster,
		})
		return getOIDCConfig(name)
	}).(pulumi.StringMapOutput)

	if _, err := s3.NewBucketObject(c.pulumiContext, fmt.Sprintf("%s-discovery", c.name), &s3.BucketObjectArgs{
		Acl:     s3.CannedAclPublicRead,
		Bucket:  bucket.ID(),
		Content: oidcConfig.MapIndex(pulumi.String(oidc.DiscoveryJSON)),
		Key:     pulumi.String(oidc.OpenIDDiscoveryPath),
		Tags:    commonAwsResourceTags,
	}, pulumi.Parent(bucket)); err != nil {
		return nil, err
	}

	if _, err := s3.NewBucketObject(c.pulumiContext, fmt.Sprintf("%s-jwks", c.name), &s3.BucketObjectArgs{
		Acl:     s3.CannedAclPublicRead,
		Bucket:  bucket.ID(),
		Content: oidcConfig.MapIndex(pulumi.String(oidc.KeysJSON)),
		Key:     pulumi.String(oidc.KeysJSON),
		Tags:    commonAwsResourceTags,
	}, pulumi.Parent(bucket)); err != nil {
		return nil, err
	}

	irsaApp := irsa.NewIRSAConfig(c.pulumiContext, c.name, cluster.Kubeconfig, kindResource)
	irsaResource, err := irsaApp.Create()
	if err != nil {
		return nil, err
	}

	cfg := pulumiconfig.New(c.pulumiContext, "")
	if cfg.Get("createSampleApp") == "true" {
		sampleAppConfig := sampleapp.NewSampleAppConfig(c.pulumiContext, bucket.BucketRegionalDomainName, openIDProvider.Arn, cluster.Kubeconfig, kindResource, []pulumi.Resource{irsaResource})
		if _, err := sampleAppConfig.Create(); err != nil {
			return nil, err
		}

	}

	return cluster, nil
}

func toKubeadmConfigPatchYAML(issuerURL string) (string, error) {
	clusterConfig := &kubeadmv1beta2.ClusterConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind: kubeadmconstants.ClusterConfigurationKind,
		},
		APIServer: kubeadmv1beta2.APIServer{
			ControlPlaneComponent: kubeadmv1beta2.ControlPlaneComponent{
				ExtraArgs: map[string]string{
					"api-audiences":            "https://kubernetes.default.svc.cluster.local,sts.amazonaws.com",
					"service-account-issuer":   fmt.Sprintf("https://%s", issuerURL),
					"service-account-jwks-uri": fmt.Sprintf("https://%s/%s", issuerURL, oidc.KeysJSON),
				},
			},
		},
	}
	clusterConfigBytes, err := json.Marshal(clusterConfig)
	return string(clusterConfigBytes), errors.Wrapf(err, "failed to marshal kubeadm cluster config yaml")
}

func getOIDCConfig(clusterName string) (map[string]string, error) {
	oidcConfig := map[string]string{}
	nodes, err := getNodes(clusterName)
	if err != nil {
		return oidcConfig, err
	}

	cpNode, err := getControlPlaneNode(nodes)
	if err != nil {
		return oidcConfig, err
	}
	cmdArgs := []string{
		"--kubeconfig",
		"/etc/kubernetes/admin.conf",
		"get",
		"--raw",
	}
	jwksJSON, err := runCommand(cpNode, "kubectl", append(cmdArgs, oidc.JWKSDiscoveryPath))
	if err != nil {
		return oidcConfig, err
	}
	discoveryJSON, err := runCommand(cpNode, "kubectl", append(cmdArgs, oidc.OpenIDDiscoveryPath))
	if err != nil {
		return oidcConfig, err
	}

	oidcConfig[oidc.DiscoveryJSON] = discoveryJSON
	oidcConfig[oidc.KeysJSON] = jwksJSON
	return oidcConfig, nil
}

func getNodes(clusterName string) ([]nodes.Node, error) {
	// just trying to auto-detect the provider
	// this might fail when the pulumi kind
	// `provider` is set to a provider that is not auto-detected
	kindclusterProvider, err := kindcluster.DetectNodeProvider()
	if err != nil {
		return []nodes.Node{}, err
	}
	prov := kindcluster.NewProvider(kindclusterProvider)
	nodes, err := prov.ListNodes(clusterName)
	return nodes, errors.Wrapf(err, fmt.Sprintf("unable to find a cluster: %s", clusterName))
}

func getControlPlaneNode(nodes []nodes.Node) (nodes.Node, error) {
	if len(nodes) < 1 {
		return nil, errors.New("cannot find any control plane nodes, is the kind cluster running...?")
	}
	cpNodes, err := nodeutils.ControlPlaneNodes(nodes)
	return cpNodes[0], errors.Wrapf(err, "unable to find a control plane node for cluster")
}

func runCommand(node nodes.Node, command string, args []string) (string, error) {
	var buff bytes.Buffer
	if err := node.Command(command, args...).SetStdout(&buff).Run(); err != nil {
		errMessage := fmt.Sprintf("error executing command: %s %s", command, strings.Join(args, " "))
		return "", errors.Wrapf(err, errMessage)
	}
	return buff.String(), nil
}
