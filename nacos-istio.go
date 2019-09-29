package main

import (
	"./common"
	"./service"
	"flag"
	"log"
	"net/http"
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

	flag.Parse()

	mockParams := &common.MockParams{
		Mocked:               *mocked,
		MockServiceCount:     *mockServiceCount,
		MockAvgEndpointCount: *mockAvgEndpointCount,
		MockPushDelay:        *mockPushDelay,
	}

	a := service.NewService(*grpcAddr, *mockParams)

	log.Println("Starting", a, "mock:", mockParams, "mocked:", *mocked)

	_ = http.ListenAndServe(*httpAddr, nil)
}
