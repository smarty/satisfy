package contracts

type DependencyListing struct {
	Dependencies []Dependency `json:"dependencies"`
}

type Dependency struct {
	Name           string `json:"name"`
	Version        string `json:"version"`
	RemoteAddress  string `json:"remote_address"`
	LocalDirectory string `json:"local_directory"`
}
