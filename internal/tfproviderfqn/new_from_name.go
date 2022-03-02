package tfproviderfqn

import (
	"fmt"
	"strings"
)

const prefix = "terraform-provider-"

func NewFromName(name string) (TfProviderFQN, error) {
	if !strings.HasPrefix(name, prefix) {
		return TfProviderFQN{}, fmt.Errorf("name must have prefix: %s", prefix)
	}

	return TfProviderFQN{
		Hostname:  defaultRegistry,
		Namespace: defaultNamespace,
		Type:      name[len(prefix):],
	}, nil
}
