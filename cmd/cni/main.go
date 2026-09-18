package main

import (
	"fmt"
	"log"

	"github.com/Wasay1567/zero-copy/pkg/network"
)

func main() {
	if err := network.ConfigureNetNS("/run/netns/testns"); err != nil {
		log.Fatal(err)
	}

	fmt.Println("network namespace configured successfully")
}