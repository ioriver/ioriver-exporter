package api

import (
	ioriver "github.com/ioriver/ioriver-go"
)

type ServiceInfo struct {
	Id   string
	Name string
}

type IORiverClient interface {
	ListServices() ([]ioriver.Service, error)
	GetTraffic(serviceId string, startTime int64, endTime int64, granularity ioriver.Granularity) (*ioriver.Traffic, error)
	GetAdvancedTraffic(serviceId string, startTime int64, endTime int64, granularity ioriver.Granularity, advancedMetric ioriver.AdvancedMetric) (*ioriver.Traffic, error)
}
