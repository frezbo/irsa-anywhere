package irsa

import (
	"github.com/frezbo/irsa-anywhere/pkg/component"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type irsaConfig struct {
	pulumiContext *pulumi.Context
	name          string
	kubeconfig    pulumi.StringInput
	parent        *component.DynamicComponent
}
