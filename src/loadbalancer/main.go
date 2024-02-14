package main

import "loadbalancer/gateway"

func main() {
	gtw := gateway.NewGateway()

	if backend := gtw.NextServer(); backend != nil {
		// do processing
	}
}
