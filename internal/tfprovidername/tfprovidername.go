package tfprovidername

import "fmt"

const (
	defaultRegistry  = "registry.terraform.io"
	defaultNamespace = "hashicorp"
)

type TfProviderName struct {
	Hostname  string
	Namespace string
	Type      string
}

func (t TfProviderName) String() string {
	return fmt.Sprintf("%s/%s/%s", t.Hostname, t.Namespace, t.Type)
}
