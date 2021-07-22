package sampleapp

import (
	"fmt"
	"strings"

	awsmeta "github.com/frezbo/irsa-anywhere/pkg/aws/meta"
	"github.com/frezbo/irsa-anywhere/pkg/component"
	"github.com/frezbo/irsa-anywhere/pkg/resource"
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/s3"
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1"
	v1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	appName = "sampleapp"
)

func NewSampleAppConfig(ctx *pulumi.Context, oidcEndpoint, oidcArn, kubeconfig pulumi.StringInput, component *component.DynamicComponent, deps []pulumi.Resource) resource.Resource {
	return &sampleAppConfig{
		pulumiContext: ctx,
		name:          appName,
		oidcEndpoint:  oidcEndpoint,
		oidcArn:       oidcArn,
		kubeconfig:    kubeconfig,
		parent:        component,
		dependencies:  deps,
	}
}

func (c *sampleAppConfig) Create() (pulumi.Resource, error) {
	// TODO: remove, added for testing
	kubeconfig := c.kubeconfig.ToStringOutput().ApplyT(func(kubeconfig string) string {
		return strings.ReplaceAll(kubeconfig, "0.0.0.0", "192.168.122.157")
	}).(pulumi.StringOutput)
	kubeProvider, err := kubernetes.NewProvider(c.pulumiContext, c.name, &kubernetes.ProviderArgs{
		Kubeconfig: kubeconfig,
	}, pulumi.Parent(c.parent), pulumi.DependsOn(c.dependencies))
	if err != nil {
		return nil, err
	}

	k8sResourceOptions := []pulumi.ResourceOption{
		pulumi.Parent(kubeProvider),
		pulumi.Provider(kubeProvider),
	}

	resourceLabels := commonLabels(c.name)

	ns, err := corev1.NewNamespace(c.pulumiContext, c.name, &corev1.NamespaceArgs{
		Metadata: v1.ObjectMetaArgs{
			Labels: resourceLabels,
			Name:   pulumi.String("irsa-test"),
		},
	}, k8sResourceOptions...)
	if err != nil {
		return nil, err
	}

	nsk8sResourceOpts := []pulumi.ResourceOption{
		pulumi.Parent(ns),
		pulumi.Provider(kubeProvider),
	}

	permissionBoundaryPolicyDocument, err := iam.GetPolicyDocument(c.pulumiContext, &iam.GetPolicyDocumentArgs{
		Statements: []iam.GetPolicyDocumentStatement{
			{
				Sid: &[]string{"allowS3GetObject"}[0],
				Actions: []string{
					"s3:ListBucket",
					"s3:Get*",
				},
				Resources: []string{"*"},
			},
		},
	}, pulumi.Parent(c.parent))
	if err != nil {
		return nil, err
	}

	commonAwsResourceTags, err := awsmeta.ResourceTags(c.pulumiContext, c.name)
	if err != nil {
		return nil, err
	}

	policy, err := iam.NewPolicy(c.pulumiContext, fmt.Sprintf("%s-permission-boundary", c.name), &iam.PolicyArgs{
		Description: pulumi.Sprintf("Permission boundary for the role"),
		Path:        pulumi.String("/"),
		Policy:      pulumi.String(permissionBoundaryPolicyDocument.Json),
		Tags:        commonAwsResourceTags,
	}, pulumi.Parent(c.parent))
	if err != nil {
		return nil, err
	}

	assumeRolePolicyDocument := pulumi.All(c.oidcArn, c.oidcEndpoint, ns.Metadata.Name().Elem()).ApplyT(func(args []interface{}) (string, error) {
		oidcArn := args[0].(string)
		oidcEndpoint := args[1].(string)
		namespace := args[2].(string)

		assumeRolePolicy, err := iam.GetPolicyDocument(c.pulumiContext, &iam.GetPolicyDocumentArgs{
			Statements: []iam.GetPolicyDocumentStatement{
				{
					Sid:     &[]string{"allowK8sServiceAccount"}[0],
					Actions: []string{"sts:AssumeRoleWithWebIdentity"},
					Principals: []iam.GetPolicyDocumentStatementPrincipal{
						{
							Type:        "Federated",
							Identifiers: []string{oidcArn},
						},
					},
					Conditions: []iam.GetPolicyDocumentStatementCondition{
						{
							Test:     "StringEquals",
							Variable: fmt.Sprintf("%s:sub", oidcEndpoint),
							Values: []string{
								fmt.Sprintf("system:serviceaccount:%s:irsa-test", namespace),
							},
						},
					},
				},
			},
		}, pulumi.Parent(c.parent))
		if err != nil {
			return "", err
		}
		return assumeRolePolicy.Json, nil
	})

	role, err := iam.NewRole(c.pulumiContext, c.name, &iam.RoleArgs{
		AssumeRolePolicy:    assumeRolePolicyDocument,
		Description:         pulumi.String("Allow a local kind cluster read only access to s3"),
		Path:                pulumi.String("/"),
		PermissionsBoundary: policy.Arn,
		Tags:                commonAwsResourceTags,
	}, pulumi.Parent(c.parent))
	if err != nil {
		return nil, err
	}

	sa, err := corev1.NewServiceAccount(c.pulumiContext, c.name, &corev1.ServiceAccountArgs{
		Metadata: v1.ObjectMetaArgs{
			Labels: resourceLabels,
			Annotations: pulumi.StringMap{
				"eks.amazonaws.com/role-arn":               role.Arn,
				"eks.amazonaws.com/audience":               pulumi.String("sts.amazonaws.com"),
				"eks.amazonaws.com/sts-regional-endpoints": pulumi.String("true"),
				"eks.amazonaws.com/token-expiration":       pulumi.String("86400"),
			},
			Name:      pulumi.String("irsa-test"),
			Namespace: ns.Metadata.Name().Elem(),
		},
	}, nsk8sResourceOpts...)
	if err != nil {
		return nil, err
	}

	bucket, err := s3.NewBucket(c.pulumiContext, c.name, &s3.BucketArgs{
		Tags: commonAwsResourceTags,
	}, pulumi.Parent(c.parent))
	if err != nil {
		return nil, err
	}

	bucketObject, err := s3.NewBucketObject(c.pulumiContext, c.name, &s3.BucketObjectArgs{
		Acl:     s3.CannedAclPublicRead,
		Bucket:  bucket.ID(),
		Content: pulumi.String("Hey,\n\nthis means your kind cluster is successfully able to talk to AWS\nwithout any long lived credentials, using aws pod identity webhook.\n\nHappy hacking\n"),
		Key:     pulumi.String(appName),
		Tags:    commonAwsResourceTags,
	}, pulumi.Parent(bucket))
	if err != nil {
		return nil, err
	}

	policyDocument := bucket.Bucket.ApplyT(func(name string) (string, error) {
		policy, err := iam.GetPolicyDocument(c.pulumiContext, &iam.GetPolicyDocumentArgs{
			Statements: []iam.GetPolicyDocumentStatement{
				{
					Actions: []string{
						"s3:ListBucket",
					},
					Resources: []string{
						fmt.Sprintf("arn:aws:s3:::%s", name),
					},
				},
				{
					Actions: []string{
						"s3:Get*",
					},
					Resources: []string{
						fmt.Sprintf("arn:aws:s3:::%s/*", name),
					},
				},
			},
		})
		if err != nil {
			return "", err
		}
		return policy.Json, nil
	})

	rolePolicy, err := iam.NewPolicy(c.pulumiContext, c.name, &iam.PolicyArgs{
		Description: pulumi.Sprintf("Allow access to read contents of bucket ", bucket.Bucket),
		Path:        pulumi.String("/"),
		Policy:      policyDocument,
		Tags:        commonAwsResourceTags,
	}, pulumi.Parent(c.parent))
	if err != nil {
		return nil, err
	}

	policyAttachment, err := iam.NewPolicyAttachment(c.pulumiContext, c.name, &iam.PolicyAttachmentArgs{
		Roles: pulumi.Array{
			role.Name,
		},
		PolicyArn: rolePolicy.Arn,
	}, pulumi.Parent(c.parent))
	if err != nil {
		return nil, err
	}

	podResourceOpts := append(nsk8sResourceOpts, pulumi.DependsOn([]pulumi.Resource{policyAttachment}))

	pod, err := corev1.NewPod(c.pulumiContext, c.name, &corev1.PodArgs{
		Metadata: v1.ObjectMetaArgs{
			Labels:    resourceLabels,
			Namespace: ns.Metadata.Name().Elem(),
		},
		Spec: corev1.PodSpecArgs{
			Containers: corev1.ContainerArray{
				corev1.ContainerArgs{
					Command: pulumi.StringArray{
						pulumi.String("/bin/bash"),
					},
					Args: pulumi.StringArray{
						pulumi.String("-c"),
						pulumi.Sprintf("aws s3 ls s3://%s && aws s3 cp s3://%s/%s . && echo -e $(cat %s)", bucket.Bucket, bucket.Bucket, bucketObject.Key, bucketObject.Key),
					},
					Image:           pulumi.String("amazon/aws-cli"),
					ImagePullPolicy: pulumi.String("Always"),
					Name:            pulumi.String("irsa-test"),
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
				},
			},
			RestartPolicy:      pulumi.String("Never"),
			ServiceAccountName: sa.Metadata.Name().Elem(),
		},
	}, podResourceOpts...)
	if err != nil {
		return nil, err
	}
	return pod, nil
}

func commonLabels(instance string) pulumi.StringMap {
	// not setting the `app.kubernetes.io/managed-by`
	// label since pulumi already sets that
	labels := map[string]string{
		"app.kubernetes.io/name":      "sampleapp",
		"app.kubernetes.io/instance":  fmt.Sprintf("sampleapp-%s", instance),
		"app.kubernetes.io/version":   "0.0.1",
		"app.kubernetes.io/component": "test",
		"app.kubernetes.io/part-of":   "irsa-test",
		// can't find a way to get metadata info from k8s
		// about the current authorized client
		"app.kubernetes.io/created-by": "pulumi",
	}
	return pulumi.ToStringMap(labels)
}
