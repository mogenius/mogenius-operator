package socketclient

import (
	"fmt"
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
	"github.com/schollz/progressbar/v3"

	"github.com/gorilla/websocket"

	mokubernetes "mogenius-k8s-manager/kubernetes"

	punqStructs "github.com/mogenius/punq/structs"
	punqUtils "github.com/mogenius/punq/utils"
	log "github.com/sirupsen/logrus"
)

func StartK8sManager() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	if utils.CONFIG.Kubernetes.RunInCluster {
		utils.PrintVersionInfo()
		utils.PrintSettings()
	} else {
		log.Infof("\n%s\n###   CURRENT CONTEXT: %s   ###\n%s\n", punqUtils.FillWith("", 90, "#"), punqUtils.FillWith(mokubernetes.CurrentContextName(), 61, " "), punqUtils.FillWith("", 90, "#"))
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
						log.Errorf("%s -> %s", &structs.EventConnectionUrl, err.Error())
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
	bar := progressbar.DefaultSilent(0)

	maxGoroutines := 10
	semaphoreChan := make(chan struct{}, maxGoroutines)
	var wg sync.WaitGroup

	defer func() {
		close(done)
	}()
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Errorf("%s -> %s", &structs.JobConnectionUrl, err.Error())
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
					log.Errorf("Cannot open uploadfile: '%s'.", err.Error())
				}
				if preparedFileRequest != nil {
					bar = progressbar.DefaultBytes(preparedFileRequest.SizeInBytes)
				} else {
					progressbar.DefaultBytes(0)
				}
			}
			if strings.HasPrefix(rawDataStr, "######END_UPLOAD######;") {
				openFile.Close()
				if preparedFileName != nil && preparedFileRequest != nil {
					services.Uploaded(*preparedFileName, *preparedFileRequest)
				}
				bar.Finish()
				os.Remove(*preparedFileName)

				var ack = structs.CreateDatagramAck("ack:files/upload:end", preparedFileRequest.Id)
				ack.Send()

				preparedFileName = nil
				preparedFileRequest = nil

				continue
			}
			if preparedFileName != nil {
				openFile.Write([]byte(rawDataStr))
				bar.Add(len(rawDataStr))
			} else {
				datagram := structs.CreateEmptyDatagram()

				var json = jsoniter.ConfigCompatibleWithStandardLibrary
				jsonErr := json.Unmarshal([]byte(rawDataStr), &datagram)
				if jsonErr != nil {
					log.Errorf("%s", jsonErr.Error())
				}
				validationErr := utils.ValidateJSON(datagram)
				if validationErr != nil {
					log.Errorf("Received malformed Datagram: %s", datagram.Pattern)
					continue
				}

				datagram.DisplayReceiveSummary()

				if isSuppressed := punqUtils.Contains(structs.SUPPRESSED_OUTPUT_PATTERN, datagram.Pattern); !isSuppressed {
					if utils.CONFIG.Misc.Debug {
						log.Info(utils.PrettyPrintInterface(datagram))
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
					log.Errorf("Pattern not found: '%s'.", datagram.Pattern)
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
	log.Info("Checking for updates ...")

	if !punqUtils.IsProduction() {
		log.Warn(" (skipped) [not production].")
		return
	}

	helmData, err := utils.GetVersionData(utils.CONFIG.Misc.HelmIndex)
	if err != nil {
		log.Errorf("GetVersionData ERR: %s", err.Error())
		return
	}
	// VALIDATE RESPONSE
	if len(helmData.Entries) < 1 {
		log.Errorf("\nHelmIndex Entries length <= 0. Check the HelmIndex for errors: %s\n", utils.CONFIG.Misc.HelmIndex)
		return
	}
	mogeniusPlatform, doesExist := helmData.Entries["mogenius-platform"]
	if !doesExist {
		log.Errorf("\nHelmIndex does not contain the field 'mogenius-platform'. Check the HelmIndex for errors: %s\n", utils.CONFIG.Misc.HelmIndex)
		return
	}
	if len(mogeniusPlatform) <= 0 {
		log.Errorf("\nField 'mogenius-platform' does not contain a proper version. Check the HelmIndex for errors: %s\n", utils.CONFIG.Misc.HelmIndex)
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
		log.Errorf("The umbrella chart 'mogenius-platform' does not contain a dependency for 'mogenius-k8s-manager'. Check the HelmIndex for errors: %s\n", utils.CONFIG.Misc.HelmIndex)
		return
	}

	if version.Ver != mok8smanager.Version {
		log.Warnf("\n####################################################################\n"+
			"####################################################################\n"+
			"######                  %s                ######\n"+
			"######               %s              ######\n"+
			"######                                                        ######\n"+
			"######                    Available: %s                    ######\n"+
			"######                    In-Use:    %s                    ######\n"+
			"######                                                        ######\n"+
			"######   %s   ######\n", color.RedString("Not updating might result in service interruption.")+
			"####################################################################\n"+
			"####################################################################\n", color.BlueString("NEW VERSION AVAILABLE!"), color.YellowString(" UPDATE AS FAST AS POSSIBLE"), color.GreenString(mok8smanager.Version), color.RedString(version.Ver))
		notUpToDateAction(helmData)
	} else {
		log.Infof(" Up-To-Date: 👍 (Your Ver: %s)\n", version.Ver)
	}
}

func notUpToDateAction(helmData *punqStructs.HelmData) {
	localVer, err := semver.NewVersion(version.Ver)
	if err != nil {
		log.Errorf("Error parsing local version: %s", err.Error())
		return
	}

	remoteVer, err := semver.NewVersion(helmData.Entries["mogenius-k8s-manager"][0].Version)
	if err != nil {
		log.Errorf("Error parsing remote version: %s", err.Error())
		return
	}

	constraint, err := semver.NewConstraint(">= " + version.Ver)
	if err != nil {
		log.Errorf("Error parsing constraint version: %s", err.Error())
		return
	}

	_, errors := constraint.Validate(remoteVer)
	for _, m := range errors {
		log.Error(m)
	}
	// Local version > Remote version (likely development version)
	if remoteVer.LessThan(localVer) {
		log.Warningf("Your local version '%s' is > the remote version '%s'. AI thinks: You are likely a developer.", localVer.String(), remoteVer.String())
		return
	}

	// MAYOR CHANGES: MUST UPGRADE TO CONTINUE
	if remoteVer.GreaterThan(localVer) && remoteVer.Major() > localVer.Major() {
		log.Fatalf("Your version '%s' is too low to continue. Please upgrade to '%s' and try again.\n", localVer.String(), remoteVer.String())
	}

	// MMINOR&PATCH CHANGES: SHOULD UPGRADE
	if remoteVer.GreaterThan(localVer) {
		log.Warningf("Your version '%s' is out-dated. Please upgrade to '%s' to avoid service interruption.", localVer.String(), remoteVer.String())
		return
	}
}
