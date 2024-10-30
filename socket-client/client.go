package socketclient

import (
	"fmt"
	"mogenius-k8s-manager/logging"
	"mogenius-k8s-manager/services"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"mogenius-k8s-manager/version"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/fatih/color"
	jsoniter "github.com/json-iterator/go"

	"github.com/gorilla/websocket"

	mokubernetes "mogenius-k8s-manager/kubernetes"

	punqStructs "github.com/mogenius/punq/structs"
	punqUtils "github.com/mogenius/punq/utils"
)

var SocketClientLogger = logging.CreateLogger("socket-client")

func StartK8sManager() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	if utils.CONFIG.Kubernetes.RunInCluster {
		utils.PrintVersionInfo()
		utils.PrintSettings()
	} else {
		message := fmt.Sprintf("\n%s\n###   CURRENT CONTEXT: %s   ###\n%s\n",
			punqUtils.FillWith("", 90, "#"),
			punqUtils.FillWith(mokubernetes.CurrentContextName(), 61, " "),
			punqUtils.FillWith("", 90, "#"),
		)
		SocketClientLogger.Info(message)
	}

	updateCheck()
	versionTicker()

	go func() {
		for status := range structs.EventConnectionStatus {
			if status {
				// CONNECTED
				for {
					_, _, err := structs.EventQueueConnection.ReadMessage()
					if err != nil {
						SocketClientLogger.Error("failed to read message for event queue", "eventConnectionUrl", structs.EventConnectionUrl, "error", err)
						break
					}
				}
				structs.EventQueueConnection.Close()
			}
		}
	}()

	for status := range structs.JobConnectionStatus {
		if status {
			// CONNECTED
			done := make(chan struct{})
			parseMessage(done, structs.JobQueueConnection)
			structs.JobQueueConnection.Close()
		}
	}
}

func parseMessage(done chan struct{}, c *websocket.Conn) {
	var preparedFileName *string
	var preparedFileRequest *services.FilesUploadRequest
	var openFile *os.File

	maxGoroutines := 100
	semaphoreChan := make(chan struct{}, maxGoroutines)
	var wg sync.WaitGroup

	defer func() {
		close(done)
	}()
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			SocketClientLogger.Error("failed to read message from websocket connection", "jobConnectionUrl", structs.JobConnectionUrl, "error", err)
			return
		} else {
			rawDataStr := string(message)
			if rawDataStr == "" {
				continue
			}
			if strings.HasPrefix(rawDataStr, "######START_UPLOAD######;") {
				preparedFileName = punqUtils.Pointer(fmt.Sprintf("%s.zip", punqUtils.NanoId()))
				rawDataStr = strings.Replace(rawDataStr, "######START_UPLOAD######;", "", 1)
				openFile, err = os.OpenFile(*preparedFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					SocketClientLogger.Error("Cannot open uploadfile", "filename", *preparedFileName, "error", err)
				}
			}
			if strings.HasPrefix(rawDataStr, "######END_UPLOAD######;") {
				openFile.Close()
				if preparedFileName != nil && preparedFileRequest != nil {
					services.Uploaded(*preparedFileName, *preparedFileRequest)
				}
				os.Remove(*preparedFileName)

				var ack = structs.CreateDatagramAck("ack:files/upload:end", preparedFileRequest.Id)
				ack.Send()

				preparedFileName = nil
				preparedFileRequest = nil

				continue
			}
			if preparedFileName != nil {
				_, err := openFile.Write([]byte(rawDataStr))
				if err != nil {
					SocketClientLogger.Error("Error writing to file", "error", err)
				}
			} else {
				datagram := structs.CreateEmptyDatagram()

				var json = jsoniter.ConfigCompatibleWithStandardLibrary
				err := json.Unmarshal([]byte(rawDataStr), &datagram)
				if err != nil {
					SocketClientLogger.Error("failed to unmarshal", "error", err)
				}
				validationErr := utils.ValidateJSON(datagram)
				if validationErr != nil {
					SocketClientLogger.Error("Received malformed Datagram", "pattern", datagram.Pattern)
					continue
				}

				datagram.DisplayReceiveSummary()

				if isSuppressed := punqUtils.Contains(structs.SUPPRESSED_OUTPUT_PATTERN, datagram.Pattern); !isSuppressed {
					if utils.CONFIG.Misc.Debug {
						SocketClientLogger.Info(utils.PrettyPrintInterface(datagram))
					}
				}

				if punqUtils.Contains(structs.COMMAND_REQUESTS, datagram.Pattern) {
					// ####### COMMAND
					semaphoreChan <- struct{}{}

					wg.Add(1)
					go func() {
						defer wg.Done()
						responsePayload := services.ExecuteCommandRequest(datagram)
						result := structs.CreateDatagramRequest(datagram, responsePayload)
						result.Send()
						<-semaphoreChan
					}()
				} else if punqUtils.Contains(structs.BINARY_REQUEST_UPLOAD, datagram.Pattern) {
					preparedFileRequest = services.ExecuteBinaryRequestUpload(datagram)

					var ack = structs.CreateDatagramAck("ack:files/upload:datagram", datagram.Id)
					ack.Send()
				} else {
					SocketClientLogger.Error("Pattern not found", "pattern", datagram.Pattern)
				}
			}
		}
	}
}

func versionTicker() {
	updateTicker := time.NewTicker(time.Second * time.Duration(utils.CONFIG.Misc.CheckForUpdates))
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
	SocketClientLogger.Info("Checking for updates ...")

	if !punqUtils.IsProduction() {
		SocketClientLogger.Warn(" (skipped) [not production].")
		return
	}

	helmData, err := utils.GetVersionData(utils.CONFIG.Misc.HelmIndex)
	if err != nil {
		SocketClientLogger.Error("GetVersionData", "error", err.Error())
		return
	}
	// VALIDATE RESPONSE
	if len(helmData.Entries) < 1 {
		SocketClientLogger.Error("HelmIndex Entries length <= 0. Check the HelmIndex for errors.", "HelmIndex", utils.CONFIG.Misc.HelmIndex)
		return
	}
	mogeniusPlatform, doesExist := helmData.Entries["mogenius-platform"]
	if !doesExist {
		SocketClientLogger.Error("HelmIndex does not contain the field 'mogenius-platform'. Check the HelmIndex for errors.", "HelmIndex", utils.CONFIG.Misc.HelmIndex)
		return
	}
	if len(mogeniusPlatform) <= 0 {
		SocketClientLogger.Error("Field 'mogenius-platform' does not contain a proper version. Check the HelmIndex for errors.", "HelmIndex", utils.CONFIG.Misc.HelmIndex)
		return
	}
	var mok8smanager *punqStructs.HelmDependency = nil
	for _, dep := range mogeniusPlatform[0].Dependencies {
		if dep.Name == "mogenius-k8s-manager" {
			mok8smanager = &dep
			break
		}
	}
	if mok8smanager == nil {
		SocketClientLogger.Error("The umbrella chart 'mogenius-platform' does not contain a dependency for 'mogenius-k8s-manager'. Check the HelmIndex for errors.", "HelmIndex", utils.CONFIG.Misc.HelmIndex)
		return
	}

	if version.Ver != mok8smanager.Version {
		message := fmt.Sprintf("\n####################################################################\n"+
			"####################################################################\n"+
			"######                  %s                ######\n"+
			"######               %s              ######\n"+
			"######                                                        ######\n"+
			"######                    Available: %s                    ######\n"+
			"######                    In-Use:    %s                    ######\n"+
			"######                                                        ######\n"+
			"######   %s   ######\n", color.RedString("Not updating might result in service interruption.")+
			"####################################################################\n"+
			"####################################################################\n",
			color.BlueString("NEW VERSION AVAILABLE!"),
			color.YellowString(" UPDATE AS FAST AS POSSIBLE"),
			color.GreenString(mok8smanager.Version),
			color.RedString(version.Ver),
		)
		SocketClientLogger.Warn(message)
		notUpToDateAction(helmData)
	} else {
		SocketClientLogger.Info(" Up-To-Date: 👍", "version", version.Ver)
	}
}

func notUpToDateAction(helmData *punqStructs.HelmData) {
	localVer, err := semver.NewVersion(version.Ver)
	if err != nil {
		SocketClientLogger.Error("Error parsing local version", "error", err)
		return
	}

	remoteVer, err := semver.NewVersion(helmData.Entries["mogenius-k8s-manager"][0].Version)
	if err != nil {
		SocketClientLogger.Error("Error parsing remote version", "error", err)
		return
	}

	constraint, err := semver.NewConstraint(">= " + version.Ver)
	if err != nil {
		SocketClientLogger.Error("Error parsing constraint version", "error", err)
		return
	}

	_, errors := constraint.Validate(remoteVer)
	for _, m := range errors {
		SocketClientLogger.Error("failed to validate semver constraint", "remoteVer", remoteVer, "error", m)
	}
	// Local version > Remote version (likely development version)
	if remoteVer.LessThan(localVer) {
		SocketClientLogger.Warn("Your local version is greater than the remote version. AI thinks: You are likely a developer.",
			"localVer", localVer.String(),
			"remoteVer", remoteVer.String(),
		)
		return
	}

	// MAYOR CHANGES: MUST UPGRADE TO CONTINUE
	if remoteVer.GreaterThan(localVer) && remoteVer.Major() > localVer.Major() {
		SocketClientLogger.Error("Your version is too low to continue. Please upgrade to and try again.\n",
			"localVer", localVer.String(),
			"remoteVer", remoteVer.String(),
		)
		panic(1)
	}

	// MINOR&PATCH CHANGES: SHOULD UPGRADE
	if remoteVer.GreaterThan(localVer) {
		SocketClientLogger.Warn("Your version is out-dated. Please upgrade to avoid service interruption.",
			"localVer", localVer.String(),
			"remoteVer", remoteVer.String(),
		)
		return
	}
}
