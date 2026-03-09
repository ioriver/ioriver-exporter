package tests

import (
	"encoding/json"
	"fmt"

	ioriver "github.com/ioriver/ioriver-go"
)

type FakeIorClient struct {
	TrafficResponseJson string
}

func (c *FakeIorClient) ListServices() ([]ioriver.Service, error) {
	var services []ioriver.Service
	if err := json.Unmarshal([]byte(listServicesResponse), &services); err != nil {
		return nil, fmt.Errorf("failed to load JSON: %w", err)
	}
	return services, nil
}

func (c *FakeIorClient) GetTraffic(serviceId string, startTime int64, endTime int64, granularity ioriver.Granularity) (*ioriver.Traffic, error) {
	return c.getTraffic(trafficOvertimeResponse)
}

func (c *FakeIorClient) GetAdvancedTraffic(serviceId string, startTime int64, endTime int64, granularity ioriver.Granularity, advancedMetric ioriver.AdvancedMetric) (*ioriver.Traffic, error) {
	traffic, err := c.getTraffic(trafficAdvancedResponse)
	if err != nil {
		return nil, err
	}
	if c.TrafficResponseJson == "" {
		name := advancedMetric.String()
		traffic.ServiceStats[0].Points[0].Metrics[0].AdvancedMetricName = &name
		traffic.ServiceStats[0].Points[0].Metrics[1].AdvancedMetricName = &name
	}
	return traffic, err
}

func (c *FakeIorClient) getTraffic(trafficResponseJson string) (*ioriver.Traffic, error) {
	var response []byte
	if c.TrafficResponseJson == "" {
		response = []byte(trafficResponseJson)
	} else {
		response = []byte(c.TrafficResponseJson)
	}

	var traffic ioriver.Traffic
	if err := json.Unmarshal(response, &traffic); err != nil {
		return nil, fmt.Errorf("failed to load JSON: %w", err)
	}
	return &traffic, nil
}

// fake responses
const ServiceId = "15e72be2-cb5a-4451-90a7-73e72553eb2a"

const listServicesResponse = `[
    {
        "id": "15e72be2-cb5a-4451-90a7-73e72553eb2a",
        "account": "815e84b8-d40d-4f54-a856-c7142c637076",
        "name": "ioriver",
        "description": "test",
        "certificate": "dacb47f6-0bd8-4b30-8a39-a2116184afe7",
        "service_uid": "FFJDKETTU3",
        "cname": "cname.net",
        "read_only": false,
        "service_template": null,
        "enable_extended_statistics": false,
        "modified": "2025-01-23T11:07:09.344439Z",
        "enable_complex_behavior_conditions": false,
        "active_version": 0,
        "enable_user_statistics": false,
        "enable_filter_stats_by_domain": false
    }
]`

const trafficOvertimeResponse = `{
	"serviceStats": [
		{
			"serviceID": "15e72be2-cb5a-4451-90a7-73e72553eb2a",
			"points": [
				{
					"timestamp": 1752827340000,
					"metrics": [
						{
							"providerName": "cfrnt",
							"geo": null,
							"advancedMetricName": null,
							"advancedMetricValue": null,
							"metrics": {
								"hits": 1542,
								"bytes": 3659521,
								"cachedHitsPercentage": 96.368355,
								"cachedBytesPercentage": 96.90178,
								"errorsPercentage": 1.3618677
							}
						},
						{
							"providerName": "fs",
							"geo": null,
							"advancedMetricName": null,
							"advancedMetricValue": null,
							"metrics": {
								"hits": 6171,
								"bytes": 14638086,
								"cachedHitsPercentage": 94.53897,
								"cachedBytesPercentage": 92.0581,
								"errorsPercentage": 2.2524712
							}
						}
					]
				}
			]
		}
	],
	"granularity": "MINUTE",
	"error": null
}`

const trafficAdvancedResponse = `{
	"serviceStats": [
		{
			"serviceID": "15e72be2-cb5a-4451-90a7-73e72553eb2a",
			"points": [
				{
					"timestamp": 1752827340000,
					"metrics": [
						{
							"providerName": "cfrnt",
							"geo": null,
							"advancedMetricName": null,
							"advancedMetricValue": "value",
							"metrics": {
								"hits": 15,
								"bytes": 3659
							}
						},
						{
							"providerName": "fs",
							"geo": null,
							"advancedMetricName": null,
							"advancedMetricValue": "value",
							"metrics": {
								"hits": 61,
								"bytes": 1463
							}
						}
					]
				}
			]
		}
	],
	"granularity": "MINUTE",
	"error": null
}`
