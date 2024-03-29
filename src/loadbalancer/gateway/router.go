package gateway

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Servers struct {
	Adds    []string `json:"adds"`
	Removes []string `json:"removes"`
}

func CreateServer(port int, gtw *Gateway) *http.Server {
	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: http.HandlerFunc(genLoadBalancer(gtw)),
	}
}

func CreateAdminServer(gtw *Gateway) *gin.Engine {
	r := gin.Default()

	r.GET("/servers", func(c *gin.Context) {
		servers := []string{}
		for _, server := range gtw.copyAlive() {
			servers = append(servers, server.rawUrl())
		}
		c.JSON(http.StatusOK, servers)
	})

	r.PUT("/servers", func(c *gin.Context) {
		var servers Servers

		if err := c.ShouldBind(&servers); err != nil {
			c.JSON(http.StatusBadRequest, nil)
			return
		}

		for _, a := range servers.Adds {
			gtw.Add(NewBackend(a, gtw))
		}

		for _, d := range servers.Removes {
			gtw.Remove(d)
		}

		c.JSON(http.StatusOK, nil)
	})

	return r
}
