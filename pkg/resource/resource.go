package resource

import "github.com/pulumi/pulumi/sdk/v3/go/pulumi"

type Resource interface {
	Create() (pulumi.Resource, error)
}
