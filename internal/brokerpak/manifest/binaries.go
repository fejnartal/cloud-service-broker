package manifest

type Binary struct {
	// Name holds the name of this resource. e.g. terraform-provider-google-beta
	Name string

	// Version holds the version of the resource e.g. 1.19.0
	Version string
}

func (m *Manifest) Binaries() ([]Binary, error) {
	return nil, nil
}
