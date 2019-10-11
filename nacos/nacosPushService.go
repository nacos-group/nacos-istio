package nacos

import (
	"../common"
	"fmt"
	"github.com/gogo/protobuf/types"
	"istio.io/api/mcp/v1alpha1"
	"istio.io/api/networking/v1alpha3"
	"log"
	"math/rand"
	"strconv"
	"time"
)
import "github.com/nacos-group/nacos-sdk-go/model"

type NacosService interface {
	// Subscribe all services changes in Nacos:
	SubscribeAllServices(SubscribeCallback func(resources *v1alpha1.Resources, err error))

	// Subscribe one service changes in Nacos:
	SubscribeService(ServiceName string, SubscribeCallback func(endpoints []model.SubscribeService, err error))
}

/**
 * Mocked Nacos service that sends whole set of services after a fixed delay.
 * This service tries to measure the performance of Pilot and the MCP protocol.
 */
type MockNacosService struct {
	// Running configurations:
	MockParams common.MockParams
	callbacks  []func(resources *v1alpha1.Resources, err error)
	// All mocked services:
	Resources *v1alpha1.Resources
}

func NewMockNacosService(MockParams common.MockParams) *MockNacosService {

	mns := &MockNacosService{
		MockParams: MockParams,
		callbacks:  []func(resources *v1alpha1.Resources, err error){},
	}

	mns.constructServices()

	go mns.notifyServiceChange()

	return mns
}

func (mockService *MockNacosService) SubscribeAllServices(SubscribeCallback func(resources *v1alpha1.Resources, err error)) {
	mockService.callbacks = append(mockService.callbacks, SubscribeCallback)
}

func (mockService *MockNacosService) SubscribeService(ServiceName string, SubscribeCallback func(endpoints []model.SubscribeService, err error)) {

}

/**
 * Construct all services that will be pushed to Istio
 */
func (mockService *MockNacosService) constructServices() {

	mockService.Resources = &v1alpha1.Resources{
		Collection: "istio/networking/v1alpha3/serviceentries",
	}

	port := &v1alpha3.Port{
		Number:   8080,
		Protocol: "HTTP",
		Name:     "http",
	}

	totalInstanceCount := 0

	labels := make(map[string]string)
	labels["p"] = "hessian2"
	labels["ROUTE"] = "0"
	labels["APP"] = "ump"
	labels["st"] = "na62"
	labels["v"] = "2.0"
	labels["TIMEOUT"] = "3000"
	labels["ih2"] = "y"
	labels["mg"] = "ump2_searchhost"
	labels["WRITE_MODE"] = "unit"
	labels["CONNECTTIMEOUT"] = "1000"
	labels["SERIALIZETYPE"] = "hessian"
	labels["ut"] = "UNZBMIX25G"

	for count := 0; count < mockService.MockParams.MockServiceCount; count++ {

		svcName := mockService.MockParams.MockServiceNamePrefix + "." + strconv.Itoa(count)
		se := &v1alpha3.ServiceEntry{
			Hosts:      []string{svcName + ".nacos"},
			Resolution: v1alpha3.ServiceEntry_STATIC,
			Location:   1,
			Ports:      []*v1alpha3.Port{port},
		}

		rand.Seed(time.Now().Unix())

		instanceCount := rand.Intn(10) + mockService.MockParams.MockAvgEndpointCount - 10

		//0.01% of the services have large number of endpoints:
		if count%10000 == 0 {
			instanceCount = 20000
		}

		totalInstanceCount += instanceCount

		for i := 0; i < instanceCount; i++ {

			ip := fmt.Sprintf("%d.%d.%d.%d",
				byte(i>>24), byte(i>>16), byte(i>>8), byte(i))

			endpoint := &v1alpha3.ServiceEntry_Endpoint{
				Labels: labels,
			}

			endpoint.Address = ip
			endpoint.Ports = map[string]uint32{
				"http": uint32(8080),
			}

			se.Endpoints = append(se.Endpoints, endpoint)
		}

		seAny, err := types.MarshalAny(se)
		if err != nil {
			continue
		}

		res := v1alpha1.Resource{
			Body: seAny,
			Metadata: &v1alpha1.Metadata{
				Annotations: map[string]string{
					"virtual": "1",
				},
				Name: "nacos" + "/" + svcName, // goes to model.Config.Name and Namespace - of course different syntax
			},
		}

		mockService.Resources.Resources = append(mockService.Resources.Resources, res)
	}

	log.Println("Generated", mockService.MockParams.MockServiceCount, "services.")
	log.Println("Total instance count", totalInstanceCount)
}

func (mockService *MockNacosService) notifyServiceChange() {
	for {

		for _, callback := range mockService.callbacks {
			go callback(mockService.Resources, nil)
		}

		time.Sleep(time.Duration(mockService.MockParams.MockPushDelay) * time.Second)
	}
}
