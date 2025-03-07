package structs

import (
	"fmt"
	"mogenius-k8s-manager/src/utils"
	"net/url"

	jsoniter "github.com/json-iterator/go"
)

const PingSeconds = 3

func MarshalUnmarshal(datagram *Datagram, data interface{}) {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary

	bytes, err := json.Marshal(datagram.Payload)
	if err != nil {
		datagram.Err = err.Error()
		return
	}
	err = json.Unmarshal(bytes, data)
	if err != nil {
		datagram.Err = err.Error()
	}
}

func UnmarshalJob(dst *BuildJob, data []byte) error {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(data, dst)
	if err != nil {
		return err
	}
	return nil
}

func UnmarshalBuildScanImageEntry(dst *BuildScanImageEntry, data []byte) error {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(data, dst)
	if err != nil {
		return err
	}
	return nil
}

func UnmarshalScan(dst *BuildScanResult, data []byte) error {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(data, dst)
	if err != nil {
		return err
	}
	return nil
}

func UnmarshalLog(dst *Log, data []byte) error {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(data, dst)
	if err != nil {
		return err
	}
	return nil
}

func UnmarshalJobListEntry(dst *BuildJob, data []byte) error {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(data, dst)
	if err != nil {
		return err
	}
	if dst != nil {
		for index, container := range dst.Service.Containers {
			if container.GitRepository != nil {
				u, err := url.Parse(*container.GitRepository)
				if err != nil {
					dst.Service.Containers[index].GitRepository = utils.Pointer("")
				} else {
					dst.Service.Containers[index].GitRepository = utils.Pointer(fmt.Sprintf("%s%s", u.Host, u.Path))
				}
			}
		}
	}
	return nil
}
