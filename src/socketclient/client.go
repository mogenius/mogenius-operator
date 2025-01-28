package socketclient

import (
	"fmt"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/services"
	"mogenius-k8s-manager/src/shell"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/version"
	"mogenius-k8s-manager/src/websocket"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	jsoniter "github.com/json-iterator/go"
)

func StartK8sManager(jobClient websocket.WebsocketClient) {
	updateCheck()
	versionTicker()

	go func() {
		for status := range structs.EventConnectionStatus {
			if status {
				// CONNECTED
				for {
					_, _, err := structs.EventQueueConnection.ReadMessage()
					if err != nil {
						socketClientLogger.Error("failed to read message for event queue", "eventConnectionUrl", structs.EventConnectionUrl, "error", err)
						break
					}
				}
				structs.EventQueueConnection.Close()
			}
		}
	}()

	startMessageHandler(jobClient)
}

func startMessageHandler(jobClient websocket.WebsocketClient) {
	var preparedFileName *string
	var preparedFileRequest *services.FilesUploadRequest
	var openFile *os.File

	maxGoroutines := 100
	semaphoreChan := make(chan struct{}, maxGoroutines)
	var wg sync.WaitGroup

	for !jobClient.IsTerminated() {
		_, message, err := jobClient.ReadMessage()
		if err != nil {
			socketClientLogger.Error("failed to read message from websocket connection", "error", err)
			time.Sleep(time.Second) // wait before next attempt to read
			continue
		}
		rawDataStr := string(message)
		if rawDataStr == "" {
			continue
		}
		if strings.HasPrefix(rawDataStr, "######START_UPLOAD######;") {
			preparedFileName = utils.Pointer(fmt.Sprintf("%s.zip", utils.NanoId()))
			openFile, err = os.OpenFile(*preparedFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				socketClientLogger.Error("Cannot open uploadfile", "filename", *preparedFileName, "error", err)
			}
			continue
		}
		if strings.HasPrefix(rawDataStr, "######END_UPLOAD######;") {
			openFile.Close()
			if preparedFileName != nil && preparedFileRequest != nil {
				services.Uploaded(*preparedFileName, *preparedFileRequest)
			}
			os.Remove(*preparedFileName)

			var ack = structs.CreateDatagramAck("ack:files/upload:end", preparedFileRequest.Id)
			ack.Send(jobClient)

			preparedFileName = nil
			preparedFileRequest = nil
			continue
		}

		if preparedFileName != nil {
			_, err := openFile.Write([]byte(rawDataStr))
			if err != nil {
				socketClientLogger.Error("Error writing to file", "error", err)
			}
			continue
		}

		datagram := structs.CreateEmptyDatagram()

		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		err = json.Unmarshal([]byte(rawDataStr), &datagram)
		if err != nil {
			socketClientLogger.Error("failed to unmarshal", "error", err)
		}
		validationErr := utils.ValidateJSON(datagram)
		if validationErr != nil {
			socketClientLogger.Error("Received malformed Datagram", "pattern", datagram.Pattern)
			continue
		}

		datagram.DisplayReceiveSummary()

		if isSuppressed := utils.Contains(structs.SUPPRESSED_OUTPUT_PATTERN, datagram.Pattern); !isSuppressed {
			moDebug, err := strconv.ParseBool(config.Get("MO_DEBUG"))
			assert.Assert(err == nil, err)
			if moDebug {
				socketClientLogger.Info("received datagram", "datagram", datagram)
			}
		}

		if slices.Contains(structs.COMMAND_REQUESTS, datagram.Pattern) {
			// ####### COMMAND
			semaphoreChan <- struct{}{}

			wg.Add(1)
			go func() {
				defer wg.Done()
				responsePayload := services.ExecuteCommandRequest(datagram)
				result := structs.CreateDatagramRequest(datagram, responsePayload)
				result.Send(jobClient)
				<-semaphoreChan
			}()
		} else if slices.Contains(structs.BINARY_REQUEST_UPLOAD, datagram.Pattern) {
			preparedFileRequest = services.ExecuteBinaryRequestUpload(datagram)

			var ack = structs.CreateDatagramAck("ack:files/upload:datagram", datagram.Id)
			ack.Send(jobClient)
		} else {
			socketClientLogger.Error("Pattern not found", "pattern", datagram.Pattern)
		}
	}
	socketClientLogger.Debug("api messagehandler finished as the websocket client was terminated")
}

func versionTicker() {
	interval, err := strconv.Atoi(config.Get("MO_UPDATE_INTERVAL"))
	assert.Assert(err == nil, err)
	updateTicker := time.NewTicker(time.Second * time.Duration(interval))
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-done:
				return
			case <-updateTicker.C:
				updateCheck()
			}
		}
	}()
}

func updateCheck() {
	if !utils.IsProduction() {
		socketClientLogger.Info("Skipping updates ... [not production]")
		return
	}

	socketClientLogger.Info("Checking for updates ...")

	helmData, err := utils.GetVersionData(utils.HELM_INDEX)
	if err != nil {
		socketClientLogger.Error("GetVersionData", "error", err.Error())
		return
	}
	// VALIDATE RESPONSE
	if len(helmData.Entries) < 1 {
		socketClientLogger.Error("HelmIndex Entries length <= 0. Check the HelmIndex for errors.", "HelmIndex", utils.HELM_INDEX)
		return
	}
	mogeniusPlatform, doesExist := helmData.Entries["mogenius-platform"]
	if !doesExist {
		socketClientLogger.Error("HelmIndex does not contain the field 'mogenius-platform'. Check the HelmIndex for errors.", "HelmIndex", utils.HELM_INDEX)
		return
	}
	if len(mogeniusPlatform) <= 0 {
		socketClientLogger.Error("Field 'mogenius-platform' does not contain a proper version. Check the HelmIndex for errors.", "HelmIndex", utils.HELM_INDEX)
		return
	}
	var mok8smanager *utils.HelmDependency = nil
	for _, dep := range mogeniusPlatform[0].Dependencies {
		if dep.Name == "mogenius-k8s-manager" {
			mok8smanager = &dep
			break
		}
	}
	if mok8smanager == nil {
		socketClientLogger.Error("The umbrella chart 'mogenius-platform' does not contain a dependency for 'mogenius-k8s-manager'. Check the HelmIndex for errors.", "HelmIndex", utils.HELM_INDEX)
		return
	}

	if version.Ver != mok8smanager.Version {
		fmt.Printf("\n####################################################################\n"+
			"####################################################################\n"+
			"######                  %s                ######\n"+
			"######               %s              ######\n"+
			"######                                                        ######\n"+
			"######                    Available: %s                    ######\n"+
			"######                    In-Use:    %s                    ######\n"+
			"######                                                        ######\n"+
			"######   %s   ######\n", shell.Colorize("Not updating might result in service interruption.", shell.Red)+
			"####################################################################\n"+
			"####################################################################\n",
			shell.Colorize("NEW VERSION AVAILABLE!", shell.Blue),
			shell.Colorize(" UPDATE AS FAST AS POSSIBLE", shell.Yellow),
			shell.Colorize(mok8smanager.Version, shell.Green),
			shell.Colorize(version.Ver, shell.Red),
		)
		notUpToDateAction(helmData)
	} else {
		socketClientLogger.Debug(" Up-To-Date: 👍", "version", version.Ver)
	}
}

func notUpToDateAction(helmData *utils.HelmData) {
	localVer, err := semver.NewVersion(version.Ver)
	if err != nil {
		socketClientLogger.Error("Error parsing local version", "error", err)
		return
	}

	remoteVer, err := semver.NewVersion(helmData.Entries["mogenius-k8s-manager"][0].Version)
	if err != nil {
		socketClientLogger.Error("Error parsing remote version", "error", err)
		return
	}

	constraint, err := semver.NewConstraint(">= " + version.Ver)
	if err != nil {
		socketClientLogger.Error("Error parsing constraint version", "error", err)
		return
	}

	_, errors := constraint.Validate(remoteVer)
	for _, m := range errors {
		socketClientLogger.Error("failed to validate semver constraint", "remoteVer", remoteVer, "error", m)
	}
	// Local version > Remote version (likely development version)
	if remoteVer.LessThan(localVer) {
		socketClientLogger.Warn("Your local version is greater than the remote version. AI thinks: You are likely a developer.",
			"localVer", localVer.String(),
			"remoteVer", remoteVer.String(),
		)
		return
	}

	// MAYOR CHANGES: MUST UPGRADE TO CONTINUE
	if remoteVer.GreaterThan(localVer) && remoteVer.Major() > localVer.Major() {
		socketClientLogger.Error("Your version is too low to continue. Please upgrade to and try again.\n",
			"localVer", localVer.String(),
			"remoteVer", remoteVer.String(),
		)
		shutdown.SendShutdownSignal(true)
		select {}
	}

	// MINOR&PATCH CHANGES: SHOULD UPGRADE
	if remoteVer.GreaterThan(localVer) {
		socketClientLogger.Warn("Your version is out-dated. Please upgrade to avoid service interruption.",
			"localVer", localVer.String(),
			"remoteVer", remoteVer.String(),
		)
		return
	}
}
