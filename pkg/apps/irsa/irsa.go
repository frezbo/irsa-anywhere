package irsa

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/frezbo/irsa-anywhere/pkg/component"
	"github.com/frezbo/irsa-anywhere/pkg/resource"
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes"
	admissionregistrationv1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/admissionregistration/v1"
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/meta/v1"
	rbacv1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/rbac/v1"
	"github.com/pulumi/pulumi-tls/sdk/v4/go/tls"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	awsPodIdentityVersion = "ed8c41f"
)

func NewIRSAConfig(ctx *pulumi.Context, name string, kubeconfig pulumi.StringInput, component *component.DynamicComponent) resource.Resource {
	return &irsaConfig{
		pulumiContext: ctx,
		name:          name,
		kubeconfig:    kubeconfig,
		parent:        component,
	}
}

func (c *irsaConfig) Create() (pulumi.Resource, error) {
	// TODO: remove, added for testing
	kubeconfig := c.kubeconfig.ToStringOutput().ApplyT(func(kubeconfig string) string {
		return strings.ReplaceAll(kubeconfig, "0.0.0.0", "192.168.122.157")
	}).(pulumi.StringOutput)
	kubeProvider, err := kubernetes.NewProvider(c.pulumiContext, c.name, &kubernetes.ProviderArgs{
		Kubeconfig: kubeconfig,
	}, pulumi.Parent(c.parent))
	if err != nil {
		return nil, err
	}

	resourceOpts := k8sResourceOptions(kubeProvider)
	resourceLabels := commonLabels(c.name)
	ns, err := corev1.NewNamespace(c.pulumiContext, c.name, &corev1.NamespaceArgs{
		Metadata: metav1.ObjectMetaArgs{
			Name:   pulumi.String("irsa-system"),
			Labels: resourceLabels,
		},
	}, resourceOpts...)
	if err != nil {
		return nil, err
	}

	nsResourceOpts := k8sNSResourceOptions(kubeProvider, ns)
	resourceNamespace := ns.Metadata.Name().Elem()

	sa, err := corev1.NewServiceAccount(c.pulumiContext, c.name, &corev1.ServiceAccountArgs{
		Metadata: metav1.ObjectMetaArgs{
			Name:      pulumi.String("pod-identity-webhook"),
			Labels:    resourceLabels,
			Namespace: resourceNamespace,
		},
	}, nsResourceOpts...)
	if err != nil {
		return nil, err
	}
	role, err := rbacv1.NewRole(c.pulumiContext, c.name, &rbacv1.RoleArgs{
		Metadata: metav1.ObjectMetaArgs{
			Name:      pulumi.String("pod-identity-webhook"),
			Labels:    resourceLabels,
			Namespace: resourceNamespace,
		},
		Rules: rbacv1.PolicyRuleArray{
			// rbacv1.PolicyRuleArgs{
			// 	ApiGroups: pulumi.StringArray{
			// 		pulumi.String(""),
			// 	},
			// 	Resources: pulumi.StringArray{
			// 		pulumi.String("secrets"),
			// 	},
			// 	Verbs: pulumi.StringArray{
			// 		pulumi.String("create"),
			// 	},
			// },
			rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{
					pulumi.String(""),
				},
				Resources: pulumi.StringArray{
					pulumi.String("secrets"),
				},
				Verbs: pulumi.StringArray{
					pulumi.String("get"),
					// pulumi.String("update"),
					// pulumi.String("patch"),
				},
				ResourceNames: pulumi.StringArray{
					pulumi.String("pod-identity-webhook"),
				},
			},
		},
	}, nsResourceOpts...)
	if err != nil {
		return nil, err
	}
	if _, err := rbacv1.NewRoleBinding(c.pulumiContext, c.name, &rbacv1.RoleBindingArgs{
		Metadata: metav1.ObjectMetaArgs{
			Name:      pulumi.String("pod-identity-webhook"),
			Labels:    resourceLabels,
			Namespace: resourceNamespace,
		},
		RoleRef: rbacv1.RoleRefArgs{
			ApiGroup: pulumi.String("rbac.authorization.k8s.io"),
			Kind:     pulumi.String("Role"),
			Name:     role.Metadata.Name().Elem(),
		},
		Subjects: rbacv1.SubjectArray{
			rbacv1.SubjectArgs{
				ApiGroup:  pulumi.String(""),
				Kind:      pulumi.String("ServiceAccount"),
				Name:      sa.Metadata.Name().Elem(),
				Namespace: resourceNamespace,
			},
		},
	}, nsResourceOpts...); err != nil {
		return nil, err
	}
	clusterRole, err := rbacv1.NewClusterRole(c.pulumiContext, c.name, &rbacv1.ClusterRoleArgs{
		Metadata: metav1.ObjectMetaArgs{
			Name:   pulumi.String("pod-identity-webhook"),
			Labels: resourceLabels,
		},
		Rules: rbacv1.PolicyRuleArray{
			rbacv1.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{
					pulumi.String(""),
				},
				Resources: pulumi.StringArray{
					pulumi.String("serviceaccounts"),
				},
				Verbs: pulumi.StringArray{
					pulumi.String("get"),
					pulumi.String("watch"),
					pulumi.String("list"),
				},
			},
			// rbacv1.PolicyRuleArgs{
			// 	ApiGroups: pulumi.StringArray{
			// 		pulumi.String("certificates.k8s.io"),
			// 	},
			// 	Resources: pulumi.StringArray{
			// 		pulumi.String("certificatesigningrequests"),
			// 	},
			// 	Verbs: pulumi.StringArray{
			// 		pulumi.String("create"),
			// 		pulumi.String("get"),
			// 		pulumi.String("list"),
			// 		pulumi.String("watch"),
			// 	},
			// },
		},
	}, resourceOpts...)
	if err != nil {
		return nil, err
	}
	if _, err := rbacv1.NewClusterRoleBinding(c.pulumiContext, c.name, &rbacv1.ClusterRoleBindingArgs{
		Metadata: metav1.ObjectMetaArgs{
			Name:   pulumi.String("pod-identity-webhook"),
			Labels: resourceLabels,
		},
		RoleRef: rbacv1.RoleRefArgs{
			ApiGroup: pulumi.String("rbac.authorization.k8s.io"),
			Kind:     pulumi.String("ClusterRole"),
			Name:     clusterRole.Metadata.Name().Elem(),
		},
		Subjects: rbacv1.SubjectArray{
			rbacv1.SubjectArgs{
				ApiGroup:  pulumi.String(""),
				Kind:      pulumi.String("ServiceAccount"),
				Name:      sa.Metadata.Name().Elem(),
				Namespace: resourceNamespace,
			},
		},
	}, resourceOpts...); err != nil {
		return nil, err
	}

	// even though the CertificateSigningRequest created by the pod-identity-webhook
	// deployment will create a secret, we're manually creating the secret
	// so that once https://github.com/aws/amazon-eks-pod-identity-webhook/pull/87 is
	// merged, the certs can be managed external to the pod-identity-webhook and the
	// excessive service account rbac permissions on `certificatesigningrequests.v1.certificates.k8s.io`
	// and v1.secrets can be removed
	// TODO: Remove once https://github.com/aws/amazon-eks-pod-identity-webhook/pull/87 is fixed, and use cert-manager for proper certs
	privKey, err := tls.NewPrivateKey(c.pulumiContext, c.name, &tls.PrivateKeyArgs{
		Algorithm:  pulumi.String("ECDSA"),
		EcdsaCurve: pulumi.String("P521"),
	}, pulumi.Parent(c.parent))
	if err != nil {
		return nil, err
	}

	certs, err := tls.NewSelfSignedCert(c.pulumiContext, c.name, &tls.SelfSignedCertArgs{
		AllowedUses: pulumi.StringArray{
			pulumi.String("key_encipherment"),
			pulumi.String("digital_signature"),
			pulumi.String("server_auth"),
		},
		DnsNames: pulumi.StringArray{
			pulumi.String("pod-identity-webhook"),
			pulumi.String("pod-identity-webhook.irsa-system"),
			pulumi.String("pod-identity-webhook.irsa-system.svc"),
			pulumi.String("pod-identity-webhook.irsa-system.svc.cluster.local"),
		},
		KeyAlgorithm:  pulumi.String("ECDSA"),
		PrivateKeyPem: privKey.PrivateKeyPem,
		Subjects: tls.SelfSignedCertSubjectArray{
			tls.SelfSignedCertSubjectArgs{
				CommonName: pulumi.String("pod-identity-webhook"),
			},
		},
		ValidityPeriodHours: pulumi.Int(72),
	}, pulumi.Parent(c.parent))
	if err != nil {
		return nil, err
	}

	secret, err := corev1.NewSecret(c.pulumiContext, c.name, &corev1.SecretArgs{
		Metadata: metav1.ObjectMetaArgs{
			Name:      pulumi.String("pod-identity-webhook"),
			Labels:    resourceLabels,
			Namespace: resourceNamespace,
		},
		StringData: pulumi.StringMap{
			"tls.key": privKey.PrivateKeyPem,
			"tls.crt": certs.CertPem,
		},
	}, nsResourceOpts...)
	if err != nil {
		return nil, err
	}

	svc, err := corev1.NewService(c.pulumiContext, c.name, &corev1.ServiceArgs{
		Metadata: metav1.ObjectMetaArgs{
			Name:      pulumi.String("pod-identity-webhook"),
			Labels:    resourceLabels,
			Namespace: resourceNamespace,
		},
		Spec: corev1.ServiceSpecArgs{
			Ports: corev1.ServicePortArray{
				corev1.ServicePortArgs{
					Port:       pulumi.Int(443),
					TargetPort: pulumi.Int(6443),
					Name:       pulumi.String("webhook-https"),
				},
			},
			Type:     corev1.ServiceSpecTypeClusterIP,
			Selector: resourceLabels,
		},
	}, nsResourceOpts...)
	if err != nil {
		return nil, err
	}
	_, err = appsv1.NewDeployment(c.pulumiContext, c.name, &appsv1.DeploymentArgs{
		Metadata: metav1.ObjectMetaArgs{
			Name:      pulumi.String("pod-identity-webhook"),
			Labels:    resourceLabels,
			Namespace: resourceNamespace,
		},
		Spec: appsv1.DeploymentSpecArgs{
			Replicas: pulumi.Int(1),
			Selector: metav1.LabelSelectorArgs{
				MatchLabels: resourceLabels,
			},
			Template: corev1.PodTemplateSpecArgs{
				Metadata: metav1.ObjectMetaArgs{
					Labels: resourceLabels,
				},
				Spec: corev1.PodSpecArgs{
					Containers: corev1.ContainerArray{
						corev1.ContainerArgs{
							Command: pulumi.StringArray{
								pulumi.String("/webhook"),
							},
							Args: pulumi.StringArray{
								pulumi.String("--in-cluster"),
								pulumi.String("--port=6443"),
								pulumi.Sprintf("--namespace=%s", resourceNamespace),
								pulumi.String("--service-name=pod-identity-webhook"),
								pulumi.Sprintf("--tls-secret=%s", secret.Metadata.Name().Elem()),
								pulumi.String("--annotation-prefix=eks.amazonaws.com"),
								pulumi.String("--token-audience=sts.amazonaws.com"),
								pulumi.String("--logtostderr"),
							},
							Image:           pulumi.Sprintf("amazon/amazon-eks-pod-identity-webhook:%s", awsPodIdentityVersion),
							ImagePullPolicy: pulumi.String("Always"),
							Name:            pulumi.String("pod-identity-webhook"),
							Ports: corev1.ContainerPortArray{
								corev1.ContainerPortArgs{
									ContainerPort: pulumi.Int(6443),
									Name:          pulumi.String("webhook-https"),
								},
							},
							Resources: corev1.ResourceRequirementsArgs{
								Limits: pulumi.StringMap{
									"cpu":    pulumi.String("100m"),
									"memory": pulumi.String("100Mi"),
								},
								Requests: pulumi.StringMap{
									"cpu":    pulumi.String("100m"),
									"memory": pulumi.String("100Mi"),
								},
							},
							SecurityContext: corev1.SecurityContextArgs{
								AllowPrivilegeEscalation: pulumi.BoolPtr(false),
								Capabilities: corev1.CapabilitiesArgs{
									Drop: pulumi.StringArray{
										pulumi.String("ALL"),
									},
								},
								Privileged:             pulumi.BoolPtr(false),
								ReadOnlyRootFilesystem: pulumi.Bool(true),
								RunAsGroup:             pulumi.Int(10000),
								RunAsUser:              pulumi.Int(10000),
								RunAsNonRoot:           pulumi.Bool(true),
							},
						},
					},
					SecurityContext: corev1.PodSecurityContextArgs{
						FsGroup: pulumi.Int(10000),
					},
					ServiceAccountName: sa.Metadata.Name(),
				},
			},
		},
	}, nsResourceOpts...)
	if err != nil {
		return nil, err
	}

	caBundleBase64Encoded := certs.CertPem.ApplyT(func(cert string) string {
		return base64.StdEncoding.EncodeToString([]byte(cert))
	}).(pulumi.StringOutput)

	webhook, err := admissionregistrationv1.NewMutatingWebhookConfiguration(c.pulumiContext, c.name, &admissionregistrationv1.MutatingWebhookConfigurationArgs{
		Metadata: metav1.ObjectMetaArgs{
			Name:      pulumi.String("pod-identity-webhook"),
			Labels:    resourceLabels,
			Namespace: resourceNamespace,
		},
		Webhooks: admissionregistrationv1.MutatingWebhookArray{
			admissionregistrationv1.MutatingWebhookArgs{
				AdmissionReviewVersions: pulumi.StringArray{
					pulumi.String("v1beta1"),
				},
				ClientConfig: admissionregistrationv1.WebhookClientConfigArgs{
					CaBundle: caBundleBase64Encoded,
					Service: admissionregistrationv1.ServiceReferenceArgs{
						Name:      svc.Metadata.Name().Elem(),
						Namespace: resourceNamespace,
						Path:      pulumi.String("/mutate"),
						Port:      svc.Spec.Ports().Index(pulumi.Int(0)).Port(),
					},
				},
				Rules: admissionregistrationv1.RuleWithOperationsArray{
					admissionregistrationv1.RuleWithOperationsArgs{
						ApiGroups: pulumi.StringArray{
							pulumi.String(""),
						},
						ApiVersions: pulumi.StringArray{
							pulumi.String("v1"),
						},
						Operations: pulumi.StringArray{
							pulumi.String("CREATE"),
						},
						Resources: pulumi.StringArray{
							pulumi.String("pods"),
						},
					},
				},
				SideEffects: pulumi.String("None"),
				Name:        pulumi.String("pod-identity-webhook.amazonaws.com"),
			},
		},
	}, resourceOpts...)
	if err != nil {
		return nil, err
	}

	return webhook, nil
}

func k8sResourceOptions(provider *kubernetes.Provider) []pulumi.ResourceOption {
	return []pulumi.ResourceOption{
		pulumi.Provider(provider),
		pulumi.Parent(provider),
	}
}

func k8sNSResourceOptions(provider *kubernetes.Provider, ns *corev1.Namespace) []pulumi.ResourceOption {
	return []pulumi.ResourceOption{
		pulumi.Provider(provider),
		pulumi.Parent(ns),
	}
}

func commonLabels(instance string) pulumi.StringMap {
	// not setting the `app.kubernetes.io/managed-by`
	// label since pulumi already sets that
	labels := map[string]string{
		"app.kubernetes.io/name":      "irsa",
		"app.kubernetes.io/instance":  fmt.Sprintf("irsa-%s", instance),
		"app.kubernetes.io/version":   awsPodIdentityVersion,
		"app.kubernetes.io/component": "iam",
		"app.kubernetes.io/part-of":   "aws-pod-identity",
		// can't find a way to get metadata info from k8s
		// about the current authorized client
		"app.kubernetes.io/created-by": "pulumi",
	}
	return pulumi.ToStringMap(labels)
}
