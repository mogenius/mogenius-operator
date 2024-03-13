package structs

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

type EnergyConsumptionEntry struct {
	ContainerName      string `json:"containerName"`
	ContainerNamespace string `json:"containerNamespace"`
	PodName            string `json:"podName"`
	TotalEnergyInJoule int    `json:"totalEnergyInJoule"`
}

type EnergyConsumptionResponse struct {
	Entries                   []EnergyConsumptionEntry `json:"entries"`
	Timestamp                 int64                    `json:"timestamp"`
	TotalEnergyInJoule        int                      `json:"totalEnergyInJoule"`
	TotalEnergyInJouleSince   int64                    `json:"totalEnergyInJouleSince"`
	EnergyInWatt              int                      `json:"energyInWatt"`
	SecondsBetweenInspections int                      `json:"secondsBetweenInspections"`
}

func CreateEnergyConsumptionEntry(containerName string, containerNamespace string, podName string, totalEnergyInJoule int) EnergyConsumptionEntry {
	return EnergyConsumptionEntry{
		ContainerName:      containerName,
		ContainerNamespace: containerNamespace,
		PodName:            podName,
		TotalEnergyInJoule: totalEnergyInJoule,
	}
}

const EnergyConsumptionResponseSize = 20
const EnergyConsumptionTimeInterval = 3

var CurrentEnergyConsumptionResponse []EnergyConsumptionResponse = make([]EnergyConsumptionResponse, EnergyConsumptionResponseSize)
var KeplerDaemonsetRunningSince int64 = 0

func CreateEnergyConsumptionResponse(input string, index int) *EnergyConsumptionResponse {
	var entriesMap map[string]EnergyConsumptionEntry = make(map[string]EnergyConsumptionEntry)

	re := regexp.MustCompile(`^kepler_container_joules_total\{container_id="[^"]+",container_name="([^"]+)",container_namespace="([^"]+)",.*pod_name="([^"]+)"}\s+([0-9.]+)`)

	lines := strings.Split(input, "\n")

	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if len(matches) == 5 {
			jouleValue, _ := strconv.ParseFloat(matches[4], 64)
			value, ok := entriesMap[matches[3]]
			if ok {
				value.TotalEnergyInJoule += int(jouleValue)
				entriesMap[matches[3]] = value
			} else {
				entriesMap[matches[3]] = CreateEnergyConsumptionEntry(matches[1], matches[2], matches[3], int(jouleValue))
			}
		}
	}

	// create array from map
	var entries []EnergyConsumptionEntry = []EnergyConsumptionEntry{}
	var totalEnergyInJoule int = 0
	for _, value := range entriesMap {
		entries = append(entries, value)
		totalEnergyInJoule += value.TotalEnergyInJoule
	}

	watts := 0
	if index > 0 {
		watts = (totalEnergyInJoule - CurrentEnergyConsumptionResponse[index-1].TotalEnergyInJoule) / EnergyConsumptionTimeInterval
		// if the value is too low, we take the value from the previous entry
		if watts <= 2 && index > 1 {
			watts = (totalEnergyInJoule - CurrentEnergyConsumptionResponse[index-2].TotalEnergyInJoule) / EnergyConsumptionTimeInterval
		}
	}

	sortWithByWatt(entries)

	CurrentEnergyConsumptionResponse[index] = EnergyConsumptionResponse{
		Entries:                   entries,
		Timestamp:                 time.Now().Unix(),
		EnergyInWatt:              watts,
		TotalEnergyInJoule:        totalEnergyInJoule,
		TotalEnergyInJouleSince:   KeplerDaemonsetRunningSince,
		SecondsBetweenInspections: EnergyConsumptionTimeInterval,
	}

	log.Infof("EnergyConsumptionMeasurement (%d/%d): %d entries - %d joule - %d watt \n", index+1, EnergyConsumptionResponseSize, len(entries), totalEnergyInJoule, watts)

	return &CurrentEnergyConsumptionResponse[index]
}

func sortWithByWatt(objs []EnergyConsumptionEntry) {
	sort.Slice(objs, func(i, j int) bool {
		return objs[i].TotalEnergyInJoule > objs[j].TotalEnergyInJoule
	})
}
