package main

import (
	"github.com/frezbo/irsa-anywhere/pkg/cluster/kind"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		kindProvider := kind.NewKindConfig(ctx, "kind-aws")
		if _, err := kindProvider.Create(); err != nil {
			return err
		}
		return nil
	})
}
