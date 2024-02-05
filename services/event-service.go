package services

import (
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/logger"
	"os/exec"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
)

// Order is the order of the events
type EventOrder int

const (
	EventOrderAsc EventOrder = iota
	EventOrderDesc
)

const (
	EventTypeNormal int = 1 << iota
	EventTypeWarning
)

type EventOptions struct {
	Limit int
	Order EventOrder
	Types int
}

func GetEvents(namespace, serviceName string, options *EventOptions) ([]v1.Event, error) {
	// Start timing the kubectl command execution
	kubectlStart := time.Now()

	// First command: kubectl get events
	kubectlCmd := exec.Command("kubectl", "get", "events", "-n", namespace, "-o", "json")
	// kubectlCmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath)

	kubectlOutput, err := kubectlCmd.Output()
	if err != nil {
		logger.Log.Errorf("Error executing kubectl command:", err)
		return nil, err
	}

	// Prepare the jq command
	if options == nil {
		options = &EventOptions{
			Limit: 20,
			Order: EventOrderDesc,
			Types: EventTypeWarning | EventTypeNormal,
		}
	}

	limit := options.Limit
	if limit <= 0 {
		limit = 20
	}

	reverse := ""
	if options.Order == EventOrderDesc {
		reverse = " | reverse"
	}

	var filterTypes []string
	if options.Types&EventTypeNormal != 0 {
		filterTypes = append(filterTypes, `.type == "Normal"`)
	}
	if options.Types&EventTypeWarning != 0 {
		filterTypes = append(filterTypes, `.type == "Warning"`)
	}

	// Prepare the jq command
	jqCmd := exec.Command("jq", "-r", "--arg", "SERVICENAME", serviceName, fmt.Sprintf(`.items | map(select((.involvedObject.name | startswith($SERVICENAME)) and (%s))) | sort_by(.lastTimestamp) %s | .[0:%d]`, strings.Join(filterTypes, " or "), reverse, limit))

	// Create a pipe for jq's stdin
	jqStdin, err := jqCmd.StdinPipe()
	if err != nil {
		logger.Log.Errorf("Error creating stdin pipe for jq:", err)
		return nil, err
	}

	// Write kubectl's output to jq's stdin in a separate goroutine
	go func() {
		defer jqStdin.Close()
		jqStdin.Write(kubectlOutput)
	}()

	// Run jq and capture its output
	jqOutput, err := jqCmd.CombinedOutput()
	if err != nil {
		logger.Log.Errorf("Error executing jq command:", err, string(jqOutput))
		return nil, err
	}

	// Measure and print the kubectl command execution time
	kubectlDuration := time.Since(kubectlStart)
	logger.Log.Debugf("kubectl command executed in: %s\n", kubectlDuration)

	// Start timing the JSON unmarshalling
	unmarshalStart := time.Now()

	// Print the final output
	// fmt.Println("Final output:", string(jqOutput))

	// Unmarshal the JSON output into the EventList struct
	var events []v1.Event
	if err := json.Unmarshal(jqOutput, &events); err != nil {
		logger.Log.Errorf("Error unmarshalling JSON: %v\n", err)
		return nil, err
	}

	// Measure and print the JSON unmarshalling time
	unmarshalDuration := time.Since(unmarshalStart)
	logger.Log.Debugf("JSON unmarshalling completed in: %s\n", unmarshalDuration)

	// Example usage: Print out the event details
	for _, event := range events {
		logger.Log.Debugf("Event: %v\n", event)
	}

	// Measure and print the JSON unmarshalling time
	totalDuration := time.Since(kubectlStart)
	logger.Log.Debugf("TOTAL completed in: %s\n", totalDuration)

	return events, nil
}