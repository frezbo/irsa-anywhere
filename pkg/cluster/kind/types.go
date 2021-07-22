package kind

import "github.com/pulumi/pulumi/sdk/v3/go/pulumi"

type kindConfig struct {
	pulumiContext *pulumi.Context
	name          string
}
