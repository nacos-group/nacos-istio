package main

import (
	"flag"
	"github.com/nkorange/nacos-istio/service"
	"log"
	"net/http"
)

var (
	nacosServer = flag.String("consul", "127.0.0.1:8848", "Address of Nacos server")

	grpcAddr = flag.String("grpcAddr", ":18848", "Address of the MCP server")

	httpAddr = flag.String("httpAddr", ":18849", "Address of the HTTP debug server")
)

func main() {

	//collections := make([]server.CollectionOptions, len(model.IstioConfigTypes))
	//for i, m := range model.IstioConfigTypes {
	//	collections[i].Name = m.Collection
	//	collections[i].Incremental = false
	//}
	//
	//options := server.Options{
	//	CollectionsOptions : collections,
	//
	//}
	//
	//serverOptions := server.ServerOptions{
	//
	//}
	//
	//s := server.NewServer(&options, &serverOptions)
	//
	//log.Println("Starting", s)

	a := service.NewService(*grpcAddr)

	log.Println("Starting", a)

	_ = http.ListenAndServe(*httpAddr, nil)
}
