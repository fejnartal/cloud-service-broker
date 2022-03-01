package tfprovidername

import (
	"fmt"
	"strings"
)

func NewFromProvider(provider string) (TfProviderName, error) {
	parts := strings.Split(provider, "/")
	switch len(parts) {
	case 1:
		return TfProviderName{
			Hostname:  defaultRegistry,
			Namespace: defaultNamespace,
			Type:      parts[0],
		}, nil
	case 2:
		return TfProviderName{
			Hostname:  defaultRegistry,
			Namespace: parts[0],
			Type:      parts[1],
		}, nil
	case 3:
		return TfProviderName{
			Hostname:  parts[0],
			Namespace: parts[1],
			Type:      parts[2],
		}, nil
	default:
		return TfProviderName{}, fmt.Errorf("invalid format; valid format is [<HOSTNAME>/]<NAMESPACE>/<TYPE>")
	}
}
