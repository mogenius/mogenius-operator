package socketServer

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
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/fatih/color"
	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"
	"github.com/schollz/progressbar/v3"
	"gopkg.in/yaml.v2"

	"github.com/gorilla/websocket"

	mokubernetes "mogenius-k8s-manager/kubernetes"
)

const PingSeconds = 10

func StartK8sManager(runsInCluster bool) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	if runsInCluster {
		utils.PrintVersionInfo()
		utils.PrintSettings()
	} else {
		fmt.Println(utils.FillWith("", 90, "#"))
		fmt.Printf("###   CURRENT CONTEXT: %s   ###\n", utils.FillWith(mokubernetes.CurrentContextName(), 61, " "))
		fmt.Println(utils.FillWith("", 90, "#"))
	}

	updateCheck()
	versionTicker()

	for status := range structs.JobConnectionStatus {
		if status {
			log.Println("Connection is active")
			done := make(chan struct{})
			parseMessage(done, structs.JobQueueConnection)
		} else {
			log.Println("Connection is not active")
		}
	}
}

func parseMessage(done chan struct{}, c *websocket.Conn) {
	var preparedFileName *string
	var preparedFileRequest *services.FilesUploadRequest
	var openFile *os.File
	bar := progressbar.DefaultSilent(0)

	go func() {
		defer func() {
			close(done)
		}()
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			} else {
				rawDataStr := string(message)
				if rawDataStr == "" {
					continue
				}
				if strings.HasPrefix(rawDataStr, "######START_UPLOAD######;") {
					preparedFileName = utils.Pointer(fmt.Sprintf("%s.zip", uuid.New().String()))
					rawDataStr = strings.Replace(rawDataStr, "######START_UPLOAD######;", "", 1)
					openFile, err = os.OpenFile(*preparedFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
						structs.PrettyPrint(datagram)
					}

					if utils.Contains(services.COMMAND_REQUESTS, datagram.Pattern) {
						// ####### COMMAND
						responsePayload := services.ExecuteCommandRequest(datagram)
						result := structs.CreateDatagramRequest(datagram, responsePayload)
						structs.JobSendMutex.Lock()
						result.Send()
						structs.JobSendMutex.Unlock()
					} else if utils.Contains(services.BINARY_REQUEST_UPLOAD, datagram.Pattern) {
						preparedFileRequest = services.ExecuteBinaryRequestUpload(datagram)

						var ack = structs.CreateDatagramAck("ack:files/upload:datagram", datagram.Id)
						ack.Send()
					} else {
						logger.Log.Errorf("Pattern not found: '%s'.", datagram.Pattern)
					}
				}
			}
		}
	}()

	// KEEP THE CONNECTION OPEN
	ping(done, c, &sendMutex)

	c.Close()
}

func ping(done chan struct{}, c *websocket.Conn, sendMutex *sync.Mutex) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	pingTicker := time.NewTicker(time.Second * PingSeconds)
	defer pingTicker.Stop()

	for {
		select {
		case <-done:
			return
		case <-pingTicker.C:
			err := c.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				log.Println("pingTicker ERROR:", err)
				return
			}
		case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			sendMutex.Lock()
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			sendMutex.Unlock()
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
				log.Fatal("CTRL + C pressed. Terminating.")
			case <-time.After(time.Second):
			}
			return
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
	var mok8smanager *structs.HelmDependency = nil
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

func getVersionData() (*structs.HelmData, error) {
	response, err := http.Get(utils.CONFIG.Misc.HelmIndex)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	data, _ := ioutil.ReadAll(response.Body)
	var helmData structs.HelmData
	err = yaml.Unmarshal(data, &helmData)
	if err != nil {
		return nil, err
	}
	return &helmData, nil
}

func notUpToDateAction(helmData *structs.HelmData) {
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
