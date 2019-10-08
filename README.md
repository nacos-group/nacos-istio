# nacos-istio

Nacos integrate with Istio as a MCP server


## Build
* Linux 
```CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build nacos-istio.go```

## Configure this MCP server in Istio

1. Edit the configMap of Istio:
```
 kubectl edit cm istio -n istio-system
```
2. Add this MCP server to the configSource list:
```
-- address: x.x.x.x:18848
```
3. Restart Pilot.

## Run in mock mode

This mode generates specified count of services with random names to test the function as well as the performance of MCP protocol with Pilot.

```./nacos-istio  --mock=true --mockServiceCount=100000 --mockPushDelay=30```

* mockServiceCount: generated service count, the endpoint count is about 10 times of service count.
* mockPushDelay: the interval in seconds between each service entry push to Pilot.

## Run in real mode

to be implemented.
