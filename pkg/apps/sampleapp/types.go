package sampleapp

import (
	"github.com/frezbo/irsa-anywhere/pkg/component"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type sampleAppConfig struct {
	pulumiContext *pulumi.Context
	name          string
	oidcEndpoint  pulumi.StringInput
	oidcArn       pulumi.StringInput
	kubeconfig    pulumi.StringInput
	parent        *component.DynamicComponent
	dependencies  []pulumi.Resource
}
