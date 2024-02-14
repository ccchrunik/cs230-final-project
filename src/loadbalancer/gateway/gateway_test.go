package gateway

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type closeNotifyingRecorder struct {
	*httptest.ResponseRecorder
	closeNotifyChan chan bool
}

func newCloseNotifyingRecorder() *closeNotifyingRecorder {
	return &closeNotifyingRecorder{
		httptest.NewRecorder(),
		make(chan bool, 1),
	}
}

func (cnr *closeNotifyingRecorder) CloseNotify() <-chan bool {
	return cnr.closeNotifyChan
}

func TestGatewayCore(t *testing.T) {
	gtw := NewGateway()

	assert.Equal(t, 0, len(gtw.serverPool.alives))
	assert.Equal(t, 0, len(gtw.serverPool.dead))

	serverNum := 100
	backends := []*Backend{}
	for i := 0; i < serverNum; i++ {
		backend := NewBackend(fmt.Sprintf("http://localhost:%d", 8000+i))
		backends = append(backends, backend)
		gtw.AddServer(backend)
	}

	gtw.sync()

	assert.Equal(t, serverNum, len(gtw.serverPool.alives))
	assert.Equal(t, 0, len(gtw.serverPool.dead))

	for i := 0; i < serverNum; i++ {
		assert.Equal(t, backends[i].rawUrl(), gtw.serverPool.alives[i].rawUrl())
	}

	removedBackendsIds := map[int]bool{10: true, 27: true, 43: true}
	for id := range removedBackendsIds {
		gtw.RemoveServer(backends[id])
	}

	gtw.sync()

	assert.Equal(t, serverNum-len(removedBackendsIds), len(gtw.serverPool.alives))
	assert.Equal(t, len(removedBackendsIds), len(gtw.serverPool.dead))

	next := 0
	for i, b := range backends {
		if _, ok := removedBackendsIds[i]; ok {
			continue
		}
		assert.Equal(t, b.rawUrl(), gtw.serverPool.alives[next].rawUrl())
		next++
	}

	// test repeated selection and remove
	crashedServerIds := []int{80, 88, 97}
	for _, id := range crashedServerIds {
		gtw.RemoveServer(backends[id])
		gtw.sync()
		removedBackendsIds[id] = true
		for i, b := range backends {
			if _, ok := removedBackendsIds[i]; ok {
				continue
			}
			backend := gtw.NextServer()
			assert.Equal(t, b.rawUrl(), backend.rawUrl())
		}
	}
}

// func TestCreateRouter(t *testing.T) {
// 	// Activate the httpmock library
// 	httpmock.Activate()
// 	defer httpmock.DeactivateAndReset()

// 	// Mock the servers
// 	httpmock.RegisterResponder("GET", "http://localhost:8081/test",
// 		httpmock.NewStringResponder(200, "Service 1"))
// 	httpmock.RegisterResponder("GET", "http://localhost:8082/test",
// 		httpmock.NewStringResponder(200, "Service 2"))

// 	// Create the router
// 	router := CreateRouter()

// 	// Create a request to the first route
// 	req, _ := http.NewRequest("GET", "/service1/test", nil)
// 	resp := newCloseNotifyingRecorder()

// 	// Serve the request
// 	router.ServeHTTP(resp, req)

// 	// Check the status code
// 	assert.Equal(t, http.StatusOK, resp.Code)
// 	assert.Equal(t, "Service 1", resp.Body.String())

// 	// Create a request to the second route
// 	req, _ = http.NewRequest("GET", "/service2/test", nil)
// 	resp = newCloseNotifyingRecorder()

// 	// Serve the request
// 	router.ServeHTTP(resp, req)

// 	// Check the status code
// 	assert.Equal(t, http.StatusOK, resp.Code)
// 	assert.Equal(t, "Service 2", resp.Body.String())
// }
