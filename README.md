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

```./nacos-istio  --mock=true --mockServiceCount=50 --mockAvgEndpointCount=70 --mockPushDelay=1 --mockServiceNamePrefix=mock1```

* mock: if use mock mode.
* mockServiceCount: generated service count, the endpoint count is about 10 times of service count.
* mockAvgEndpointCount: average endpoint count of each service, shouldn't be smaller than 10. (To test large endpoints number, 0.0.1% of the services will each have 20000 endpoints.)
* mockPushDelay: the interval in seconds between each service entry push to Pilot.
* mockServiceNamePrefix: service name prefix.

## Run in real mode

to be implemented.
