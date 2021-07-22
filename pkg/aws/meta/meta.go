package meta

import (
	"github.com/frezbo/irsa-anywhere/pkg/common/meta"
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func ResourceTags(ctx *pulumi.Context, resourceName string) (pulumi.StringMap, error) {
	callerIdentity, err := getCallerIdentity(ctx)
	if err != nil {
		return nil, err
	}
	// TODO: Add git context if in a git repository?
	resourceTags := pulumi.StringMap{
		// let's add a sensible auto-generated name
		"Name":       meta.GenerateResourceName(ctx, resourceName),
		"Owner":      pulumi.String(callerIdentity.Id),
		"CreatedBy":  pulumi.String("pulumi"),
		"CreatorArn": pulumi.String(callerIdentity.Arn),
		"Project":    pulumi.String(ctx.Project()),
		"Stack":      pulumi.String(ctx.Stack()),
	}
	return resourceTags, nil
}

func getCallerIdentity(ctx *pulumi.Context) (*aws.GetCallerIdentityResult, error) {
	callerIdentity, err := aws.GetCallerIdentity(ctx, nil, nil)
	if err != nil {
		return nil, err
	}
	return callerIdentity, nil
}
