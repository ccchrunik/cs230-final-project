package main

import "loadbalancer/gateway"

func main() {
	router := gateway.CreateRouter()
	gateway.StartRouter(router)
}
