package meta

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func GenerateResourceName(ctx *pulumi.Context, resourceName string) pulumi.StringOutput {
	return pulumi.Sprintf("%s-%s-%s", ctx.Stack(), ctx.Project(), resourceName)
}
