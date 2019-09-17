package nacos

import "github.com/nacos-group/nacos-sdk-go/model"

type NacosService interface {

	// Subscribe all services changes in Nacos:
	SubscribeAllServices(SubscribeCallback func(endpoints map[string][]model.SubscribeService, err error)) error

	// Subscribe one service changes in Nacos:
	SubscribeService(ServiceName string, SubscribeCallback func(endpoints []model.SubscribeService, err error))
}
