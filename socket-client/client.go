package socketclient

import (
	"fmt"
	"io/ioutil"
	"log"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/services"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"mogenius-k8s-manager/version"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/fatih/color"
	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"
	"github.com/schollz/progressbar/v3"
	"gopkg.in/yaml.v2"

	"github.com/gorilla/websocket"

	mokubernetes "mogenius-k8s-manager/kubernetes"

	punqStructs "github.com/mogenius/punq/structs"
	punqUtils "github.com/mogenius/punq/utils"
)

func StartK8sManager() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	if utils.CONFIG.Kubernetes.RunInCluster {
		utils.PrintVersionInfo()
		utils.PrintSettings()
	} else {
		fmt.Println(punqUtils.FillWith("", 90, "#"))
		fmt.Printf("###   CURRENT CONTEXT: %s   ###\n", punqUtils.FillWith(mokubernetes.CurrentContextName(), 61, " "))
		fmt.Println(punqUtils.FillWith("", 90, "#"))
	}

	updateCheck()
	versionTicker()

	for status := range structs.JobConnectionStatus {
		if status {
			// CONNECTED
			done := make(chan struct{})
			parseMessage(done, structs.JobQueueConnection)
		} else {
			// DISCONNECTED
		}
	}

	fmt.Println("omg")
}

func parseMessage(done chan struct{}, c *websocket.Conn) {
	var preparedFileName *string
	var preparedFileRequest *services.FilesUploadRequest
	var openFile *os.File
	bar := progressbar.DefaultSilent(0)

	defer func() {
		close(done)
	}()
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			logger.Log.Errorf("%s -> %s", &structs.JobConnectionUrl, err.Error())
			return
		} else {
			rawDataStr := string(message)
			if rawDataStr == "" {
				continue
			}
			if strings.HasPrefix(rawDataStr, "######START_UPLOAD######;") {
				preparedFileName = punqUtils.Pointer(fmt.Sprintf("%s.zip", uuid.New().String()))
				rawDataStr = strings.Replace(rawDataStr, "######START_UPLOAD######;", "", 1)
				openFile, err = os.OpenFile(*preparedFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					logger.Log.Errorf("Cannot open uploadfile: '%s'.", err.Error())
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
					logger.Log.Errorf("%s", jsonErr.Error())
				}

				datagram.DisplayReceiveSummary()

				if utils.CONFIG.Misc.Debug {
					punqStructs.PrettyPrint(datagram)
				}

				if punqUtils.Contains(services.COMMAND_REQUESTS, datagram.Pattern) {
					// ####### COMMAND
					go func() {
						responsePayload := services.ExecuteCommandRequest(datagram)
						result := structs.CreateDatagramRequest(datagram, responsePayload)
						result.Send()
					}()
				} else if punqUtils.Contains(services.BINARY_REQUEST_UPLOAD, datagram.Pattern) {
					preparedFileRequest = services.ExecuteBinaryRequestUpload(datagram)

					var ack = structs.CreateDatagramAck("ack:files/upload:datagram", datagram.Id)
					ack.Send()
				} else {
					logger.Log.Errorf("Pattern not found: '%s'.", datagram.Pattern)
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
	fmt.Print("Checking for updates ...")

	if !punqUtils.IsProduction() {
		fmt.Println(" (skipped) [not production].")
		return
	}

	helmData, err := getVersionData()

	if err != nil {
		logger.Log.Error(err)
		return
	}
	// VALIDATE RESPONSE
	if len(helmData.Entries) < 1 {
		fmt.Printf("\n")
		logger.Log.Errorf("HelmIndex Entries length <= 0. Check the HelmIndex for errors: %s\n", utils.CONFIG.Misc.HelmIndex)
		return
	}
	mogeniusPlatform, doesExist := helmData.Entries["mogenius-platform"]
	if !doesExist {
		fmt.Printf("\n")
		logger.Log.Errorf("HelmIndex does not contain the field 'mogenius-platform'. Check the HelmIndex for errors: %s\n", utils.CONFIG.Misc.HelmIndex)
		return
	}
	if len(mogeniusPlatform) <= 0 {
		fmt.Printf("\n")
		logger.Log.Errorf("Field 'mogenius-platform' does not contain a proper version. Check the HelmIndex for errors: %s\n", utils.CONFIG.Misc.HelmIndex)
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
		logger.Log.Errorf("The umbrella chart 'mogenius-platform' does not contain a dependency for 'mogenius-k8s-manager'. Check the HelmIndex for errors: %s\n", utils.CONFIG.Misc.HelmIndex)
		return
	}

	if version.Ver != mok8smanager.Version {
		fmt.Printf("\n")
		fmt.Printf("####################################################################\n")
		fmt.Printf("####################################################################\n")
		fmt.Printf("######                  %s                ######\n", color.BlueString("NEW VERSION AVAILABLE!"))
		fmt.Printf("######               %s              ######\n", color.YellowString(" UPDATE AS FAST AS POSSIBLE"))
		fmt.Printf("######                                                        ######\n")
		fmt.Printf("######                    Available: %s                    ######\n", color.GreenString(mok8smanager.Version))
		fmt.Printf("######                    In-Use:    %s                    ######\n", color.RedString(version.Ver))
		fmt.Printf("######                                                        ######\n")
		fmt.Printf("######   %s   ######\n", color.RedString("Not updating might result in service interruption."))
		fmt.Printf("####################################################################\n")
		fmt.Printf("####################################################################\n")
		notUpToDateAction(helmData)
	} else {
		fmt.Printf(" Up-To-Date: 👍 (Your Ver: %s)\n", version.Ver)
	}
}

func getVersionData() (*punqStructs.HelmData, error) {
	response, err := http.Get(utils.CONFIG.Misc.HelmIndex)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	data, _ := ioutil.ReadAll(response.Body)
	var helmData punqStructs.HelmData
	err = yaml.Unmarshal(data, &helmData)
	if err != nil {
		return nil, err
	}
	return &helmData, nil
}

func notUpToDateAction(helmData *punqStructs.HelmData) {
	localVer, err := semver.NewVersion(version.Ver)
	if err != nil {
		logger.Log.Error("Error parsing local version: %s", err.Error())
		return
	}

	remoteVer, err := semver.NewVersion(helmData.Entries["mogenius-k8s-manager"][0].Version)
	if err != nil {
		logger.Log.Error("Error parsing remote version: %s", err.Error())
		return
	}

	constraint, err := semver.NewConstraint(">= " + version.Ver)
	if err != nil {
		logger.Log.Error("Error parsing constraint version: %s", err.Error())
		return
	}

	_, errors := constraint.Validate(remoteVer)
	for _, m := range errors {
		fmt.Println(m)
	}
	// Local version > Remote version (likely development version)
	if remoteVer.LessThan(localVer) {
		logger.Log.Warningf("Your local version '%s' is > the remote version '%s'. AI thinks: You are likely a developer.", localVer.String(), remoteVer.String())
		return
	}

	// MAYOR CHANGES: MUST UPGRADE TO CONTINUE
	if remoteVer.GreaterThan(localVer) && remoteVer.Major() > localVer.Major() {
		log.Fatalf("Your version '%s' is too low to continue. Please upgrade to '%s' and try again.\n", localVer.String(), remoteVer.String())
	}

	// MMINOR&PATCH CHANGES: SHOULD UPGRADE
	if remoteVer.GreaterThan(localVer) {
		logger.Log.Warningf("Your version '%s' is out-dated. Please upgrade to '%s' to avoid service interruption.", localVer.String(), remoteVer.String())
		return
	}
}
