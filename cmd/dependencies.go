package cmd

type DependencyListing struct {
	Dependencies []Dependency `json:"dependencies"`
}

func (this *DependencyListing) Validate() error {
	return nil // TODO
}

type Dependency struct {
	Name           string `json:"name"`
	Version        string `json:"version"`
	RemoteAddress  URL    `json:"remote_address"`
	LocalDirectory string `json:"local_directory"`
}
