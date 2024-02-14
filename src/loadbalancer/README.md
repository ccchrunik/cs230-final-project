# Load Balancer

This component proxys the request of clients to multiple backend server. The load balancer has several policies to choose from based on the server loads.

When detecting a server failure, the load balancer will remove the server from the candidate list. It also receives messages from the monitor to connect to a new instance.

This component is adapted from this [tutorial](https://kasvith.me/posts/lets-create-a-simple-lb-go/). We add more policy and tests in the project.

## Reference
- https://kasvith.me/posts/lets-create-a-simple-lb-go/