package tfprovidername

import (
	"fmt"
	"strings"
)

const prefix = "terraform-provider-"

func NewFromName(name string) (TfProviderName, error) {
	if !strings.HasPrefix(name, prefix) {
		return TfProviderName{}, fmt.Errorf("name must have prefix: %s", prefix)
	}

	return TfProviderName{
		Hostname:  defaultRegistry,
		Namespace: defaultNamespace,
		Type:      name[len(prefix):],
	}, nil
}
