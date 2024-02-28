package main

import (
	"loadbalancer/gateway"
	"log"
)

func main() {
	gtw := gateway.NewGateway()
	server := gateway.CreateServer(3000, gtw)
	adminServer := gateway.CreateAdminServer(gtw)

	go func() {
		if err := adminServer.Run(":5000"); err != nil {
			log.Println(err)
		}
	}()

	// go gateway.HealthCheck(gtw, 30*time.Second)

	// go gateway.ResurrectServer(gtw, 30*time.Second)

	if err := server.ListenAndServe(); err != nil {
		log.Println(err)
	}
}
