package component

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func NewDynamicComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*DynamicComponent, error) {
	dynamicComponent := &DynamicComponent{}
	componentToken := fmt.Sprintf("irsa:cluster:%s", name)
	err := ctx.RegisterComponentResource(componentToken, name, dynamicComponent, opts...)
	if err != nil {
		return nil, err
	}
	return dynamicComponent, nil
}
