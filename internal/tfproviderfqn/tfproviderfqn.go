package tfproviderfqn

import "fmt"

const (
	defaultRegistry  = "registry.terraform.io"
	defaultNamespace = "hashicorp"
)

func New(name, provider string) (TfProviderFQN, error) {
	switch provider {
	case "":
		return NewFromName(name)
	default:
		return NewFromProvider(provider)
	}
}

type TfProviderFQN struct {
	Hostname  string
	Namespace string
	Type      string
}

func (t TfProviderFQN) String() string {
	return fmt.Sprintf("%s/%s/%s", t.Hostname, t.Namespace, t.Type)
}
