package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/nacos-group/nacos-istio/common"
	"github.com/nacos-group/nacos-istio/service"
)

var (
	nacosServer = flag.String("nacos", "127.0.0.1:8848", "Address of Nacos server")

	grpcAddr = flag.String("grpcAddr", ":18848", "Address of the MCP server")

	httpAddr = flag.String("httpAddr", ":18849", "Address of the HTTP debug server")
)

func main() {

	mocked := flag.Bool("mock", false, "If in mock mode.")

	mockServiceCount := flag.Int("mockServiceCount", 0, "service count to test, only used when --mock=true")

	mockAvgEndpointCount := flag.Int("mockAvgEndpointCount", 20, "average endpoint count for every service, only used when --mock=true")

	mockPushDelay := flag.Int64("mockPushDelay", 10, "push delay in seconds, only used when --mock=true")

	mockServiceNamePrefix := flag.String("mockServiceNamePrefix", "mock.service", "mock service name prefix")

	mockTestIncremental := flag.Bool("mockTestIncremental", false, "mock service is incremental")

	mockIncrementalRatio := flag.Int("mockIncrementalRatio", 0, "ratio of incremental push services")

	flag.Parse()

	mockParams := &common.MockParams{
		Mocked:                *mocked,
		MockServiceCount:      *mockServiceCount,
		MockAvgEndpointCount:  *mockAvgEndpointCount,
		MockPushDelay:         *mockPushDelay,
		MockServiceNamePrefix: *mockServiceNamePrefix,
		MockTestIncremental:   *mockTestIncremental,
		MockIncrementalRatio:  *mockIncrementalRatio,
	}

	a := service.NewService(*grpcAddr, *mockParams)

	log.Println("Starting", a, "mock:", mockParams, "mocked:", *mocked)

	_ = http.ListenAndServe(*httpAddr, nil)
}
