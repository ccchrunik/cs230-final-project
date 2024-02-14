package gateway

import (
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/emirpasic/gods/sets/hashset"
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

func TestGatewayCoreSequential(t *testing.T) {
	gtw := NewGateway()

	assert.Equal(t, 0, len(gtw.serverPool.alives))
	assert.Equal(t, 0, len(gtw.serverPool.dead))

	serverNum := 10000
	backends := []*Backend{}
	for i := 0; i < serverNum; i++ {
		backend := NewBackend(fmt.Sprintf("http://localhost:%d", 8000+i))
		backends = append(backends, backend)
		gtw.AddServer(backend)
	}

	assert.Equal(t, serverNum, len(gtw.serverPool.alives))
	assert.Equal(t, 0, len(gtw.serverPool.dead))

	for i := 0; i < serverNum; i++ {
		assert.Equal(t, backends[i].rawUrl(), gtw.serverPool.alives[i].rawUrl())
	}

	removedBackendsIds := map[int]bool{10: true, 27: true, 43: true}
	for id := range removedBackendsIds {
		gtw.RemoveServer(backends[id].rawUrl())
	}

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
		gtw.RemoveServer(backends[id].rawUrl())
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

func TestGatewayCoreConcurrent(t *testing.T) {
	gtw := NewGateway()

	assert.Equal(t, 0, len(gtw.serverPool.alives))
	assert.Equal(t, 0, len(gtw.serverPool.dead))

	serverNum := 10000
	backendSet := hashset.New()
	for i := 0; i < serverNum; i++ {
		backendSet.Add(fmt.Sprintf("http://localhost:%d", 8000+i))
	}

	var wg sync.WaitGroup
	wg.Add(serverNum)
	for _, rawUrl := range backendSet.Values() {
		go func(rawUrl string) {
			backend := NewBackend(rawUrl)
			gtw.AddServer(backend)
			wg.Done()
		}(rawUrl.(string))
	}
	wg.Wait()

	assert.Equal(t, serverNum, backendSet.Size())
	assert.Equal(t, serverNum, len(gtw.serverPool.alives))
	assert.Equal(t, 0, len(gtw.serverPool.dead))

	servers := gtw.copy()
	for i := 0; i < serverNum; i++ {
		assert.True(t, backendSet.Contains(servers[i].rawUrl()))
	}

	removedNum := 5000
	removedSet := hashset.New()
	for i := 0; i < removedNum; i++ {
		removedSet.Add(fmt.Sprintf("http://localhost:%d", 8000+i))
	}

	wg.Add(removedNum)
	for _, rawUrl := range removedSet.Values() {
		go func(rawUrl string) {
			gtw.RemoveServer(rawUrl)
			wg.Done()
		}(rawUrl.(string))
	}
	wg.Wait()

	assert.Equal(t, serverNum-removedNum, len(gtw.serverPool.alives))
	assert.Equal(t, removedNum, len(gtw.serverPool.dead))

	remainingServers := gtw.copy()
	remainingSet := backendSet.Difference(removedSet)
	for i := 0; i < len(remainingServers); i++ {
		rawUrl := remainingServers[i].rawUrl()
		assert.True(t, remainingSet.Contains(rawUrl))
		assert.False(t, removedSet.Contains(rawUrl))
	}

	// test repeated selection and remove
	newRemovedNum := 100
	newRemovedServers := hashset.New()
	for i := 0; i < newRemovedNum; i++ {
		newRemovedServers.Add(fmt.Sprintf("http://localhost:%d", 8000+removedNum+10*i))
	}

	for _, v := range newRemovedServers.Values() {
		rawUrl := v.(string)
		gtw.RemoveServer(rawUrl)
		remainingSet.Remove(rawUrl)
		slen := gtw.serverPool.Len()

		assert.Equal(t, remainingSet.Size(), slen)
		resultSet := hashset.New()
		resultCh := make(chan string)
		doneCh := make(chan struct{})
		wg.Add(slen)
		for i := 0; i < slen; i++ {
			go func() {
				resultCh <- gtw.NextServer().rawUrl()
				wg.Done()
			}()
		}

		go func() {
			for i := 0; i < slen; i++ {
				resultSet.Add(<-resultCh)
			}
			doneCh <- struct{}{}
		}()

		<-doneCh
		wg.Wait()

		// all selections should contains the entire sets of remaing servers
		assert.Equal(t, remainingSet.Size(), resultSet.Size())
		diff := remainingSet.Difference(resultSet)
		assert.True(t, diff.Empty())
	}
}

// func TestGatewayHTTP(t *testing.T) {
// 	gtw := NewGateway()

// 	serverNum := 100
// 	backends := []*Backend{}
// 	for i := 0; i < serverNum; i++ {
// 		backend := NewBackend(fmt.Sprintf("http://localhost:%d", 8000+i))
// 		backends = append(backends, backend)
// 		gtw.AddServer(backend)
// 	}

// 	gtw.sync()

// 	httpmock.Activate()
// 	defer httpmock.DeactivateAndReset()

// 	for i := 0; i < serverNum; i++ {
// 		httpmock.RegisterResponder("GET", fmt.Sprintf("http://localhost:%d", 8000+i),
// 			httpmock.NewStringResponder(200, fmt.Sprintf("Service %d", i)))
// 	}

// 	for i := 0; i < serverNum; i++ {
// 		req, _ := http.NewRequest("GET", "/service1/test", nil)
// 		resp := newCloseNotifyingRecorder()
// 	}

// }

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
