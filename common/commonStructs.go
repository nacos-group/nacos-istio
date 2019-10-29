package common

type MockParams struct {
	Mocked                bool
	MockServiceCount      int
	MockAvgEndpointCount  int
	MockPushDelay         int64
	MockServiceNamePrefix string
	MockTestIncremental   bool
	MockIncrementalRatio  int
}
