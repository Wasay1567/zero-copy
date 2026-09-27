package cni

import current "github.com/containernetworking/cni/pkg/types/100"

// Result and Interface use the official CNI schema, including IPs, routes and DNS.
type Result = current.Result
type Interface = current.Interface

func NewResult(version string) *Result {
	return &Result{CNIVersion: version}
}
