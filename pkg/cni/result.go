package cni

type Result struct {
	CNIVersion string      `json:"cniVersion"`
	Interfaces []Interface `json:"interfaces,omitempty"`
}

type Interface struct {
	Name    string `json:"name"`
	Sandbox string `json:"sandbox,omitempty"`
}

func NewResult(version string) *Result {
	return &Result{
		CNIVersion: version,
	}
}