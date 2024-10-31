package services

import (
	"fmt"
	"math/rand"
	dbstats "mogenius-k8s-manager/db-stats"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/structs"
	"net"
	"strings"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/mogenius/punq/utils"
)

type DataPoint struct {
	ComType     string
	SrcIp       string
	SrcPort     string
	DstIp       string
	DstPort     string
	PacketCount uint64
}

type ChartPodDataRequest struct {
	Namespace string `json:"namespace"`
	PodName   string `json:"podName"`
}

func ChartPodDataRequestExamppleData() ChartPodDataRequest {
	return ChartPodDataRequest{
		Namespace: "mogenius",
		PodName:   "mogenius-k8s-manager-6fc9c8ddf5-4lb28",
	}
}

func generateTreeFromData(namespace string, serviceName string, data []DataPoint) []opts.TreeData {
	result := []opts.TreeData{}

	maxVal := findMax(data)
	sumValue := sumData(data)

	// NS Node
	ns := opts.TreeData{
		Name:       namespace,
		Symbol:     "image://https://github.com/kubernetes/community/blob/master/icons/png/resources/labeled/ns-128.png?raw=true",
		SymbolSize: 120,
		ItemStyle:  &opts.ItemStyle{ShadowColor: "#000", ShadowOffsetX: 1, ShadowOffsetY: 1, ShadowBlur: 10},
	}
	// POD Node
	main := opts.TreeData{
		Name:       serviceName,
		Symbol:     "image://https://github.com/kubernetes/community/blob/master/icons/png/resources/labeled/pod-128.png?raw=true",
		SymbolSize: 120,
		Value:      int(sumValue),
		ItemStyle:  &opts.ItemStyle{ShadowColor: "#000", ShadowOffsetX: 1, ShadowOffsetY: 1, ShadowBlur: 10},
	}

	// IP Nodes
	for _, dataPoint := range data {
		foundExisting := false
		for _, v := range main.Children {
			if v.Name == dataPoint.SrcIp {
				foundExisting = true
			}
		}

		if !foundExisting {
			itemstyle := &opts.ItemStyle{}
			if !checkIfIpIsPrivate(dataPoint.SrcIp) {
				itemstyle.ShadowColor = "#FF0000"
				itemstyle.ShadowBlur = 10
				itemstyle.ShadowOffsetX = 10
				itemstyle.ShadowOffsetY = 10
			}
			main.Children = append(main.Children, &opts.TreeData{
				Name:       dataPoint.SrcIp,
				Symbol:     "image://https://github.com/kubernetes/community/blob/master/icons/png/resources/labeled/pod-128.png?raw=true",
				Value:      int(dataPoint.PacketCount),
				Collapsed:  opts.Bool(false),
				SymbolSize: reduceToNearest(dataPoint.PacketCount, maxVal),
				LineStyle:  &opts.LineStyle{Width: 2, Color: generateRandomHexColor(), Opacity: 0.5},
				ItemStyle:  itemstyle,
			})
		}
	}
	// PORT Nodes
	for _, dataPoint := range data {
		combination := fmt.Sprintf("%s %s", dataPoint.ComType, dataPoint.SrcPort)
		var nextChild *opts.TreeData = nil
		for _, v := range main.Children {
			if v.Name == dataPoint.SrcIp {
				nextChild = v
				break
			}
		}

		if nextChild != nil {
			noDuplicate := true
			for _, v := range nextChild.Children {
				if v.Name == combination {
					noDuplicate = false
				}

			}
			if noDuplicate {
				portChild := &opts.TreeData{
					Name:       combination,
					Symbol:     "image://https://github.com/kubernetes/community/blob/master/icons/png/resources/labeled/pod-128.png?raw=true",
					Value:      int(dataPoint.PacketCount),
					Collapsed:  opts.Bool(false),
					SymbolSize: reduceToNearest(dataPoint.PacketCount, maxVal),
					LineStyle:  &opts.LineStyle{Width: float32(reduceToNearest(dataPoint.PacketCount, maxVal)), Color: generateRandomHexColor(), Opacity: 0.5},
					ItemStyle:  &opts.ItemStyle{ShadowColor: "#000", ShadowOffsetX: 1, ShadowOffsetY: 1},
				}
				for _, nextDataPoint := range data {
					if dataPoint.SrcIp == nextDataPoint.SrcIp && dataPoint.SrcPort == nextDataPoint.SrcPort {
						noSubDuplicate := true
						for _, v := range portChild.Children {
							if v.Name == dataPoint.DstIp {
								noSubDuplicate = false
							}

						}
						if noSubDuplicate {
							portChild.Children = append(portChild.Children, &opts.TreeData{
								Name:       nextDataPoint.DstIp,
								Symbol:     "image://https://github.com/kubernetes/community/blob/master/icons/png/resources/labeled/svc-128.png?raw=true",
								Value:      int(dataPoint.PacketCount),
								Collapsed:  opts.Bool(true),
								SymbolSize: 20,
								LineStyle:  &opts.LineStyle{Width: 2.0, Color: generateRandomHexColor(), Opacity: 1},
								ItemStyle:  &opts.ItemStyle{ShadowColor: "#000", ShadowOffsetX: 1, ShadowOffsetY: 1},
							})
						}
					}
				}

				nextChild.Children = append(nextChild.Children, portChild)
			}
		}
	}

	ns.Children = append(ns.Children, &main)
	result = append(result, ns)
	return result
}

func checkIfIpIsPrivate(ipString string) bool {
	ip := net.ParseIP(ipString)
	return ip.IsPrivate()
}

func sumData(data []DataPoint) uint64 {
	sum := uint64(0)
	for _, val := range data {
		sum += val.PacketCount
	}
	return sum
}

// reduceToNearest maps a uint64 value to the nearest of the predefined steps (20, 30, ..., 80).
func reduceToNearest(value uint64, maxValue uint64) uint64 {
	if maxValue == 0 {
		return 0
	}
	// Normalize the value to a 0-100 scale.
	normalized := float64(value) / float64(maxValue) * 100

	// Round the normalized value to the nearest of the predefined steps.
	nearest := uint64((normalized+5)/10) * 10 // +5 for rounding to nearest 10

	if nearest > 80 {
		nearest = 80
	}
	if nearest < 20 {
		nearest = 20
	}
	return nearest
}

func findMax(data []DataPoint) uint64 {
	max := uint64(0)
	for _, val := range data {
		if val.PacketCount > max {
			max = val.PacketCount
		}
	}
	return max
}

func generateRandomHexColor() string {
	color := fmt.Sprintf("#%06X", rand.Intn(1<<24))
	return color
}

func processDataPoints(data map[string]uint64) []DataPoint {
	result := []DataPoint{}
	for k, v := range data {
		dataStrSplit := strings.Split(k, "-")
		comType := dataStrSplit[0]
		srcSplit := strings.Split(dataStrSplit[1], ":")
		srcIp := srcSplit[0]
		srcPort := srcSplit[1]
		dstSplit := strings.Split(dataStrSplit[2], ":")
		dstIp := dstSplit[0]
		dstPort := dstSplit[1]

		result = append(result, DataPoint{
			ComType:     comType,
			SrcIp:       srcIp,
			SrcPort:     srcPort,
			DstIp:       dstIp,
			DstPort:     dstPort,
			PacketCount: v,
		})
	}
	return result
}

func generateTree(data structs.InterfaceStats, conData structs.SocketConnections, controller kubernetes.K8sController) *charts.Tree {
	title := fmt.Sprintf(""+
		"%s/%s\n"+
		"%s/%s", controller.Kind, controller.Name, data.Namespace, data.PodName)
	subtitle := fmt.Sprintf(""+
		"Packets Captured: %d\n"+
		"Hosts:            %d\n"+
		"Total Rx:         %s\n"+
		"Total Tx:         %s\n"+
		"Uptime:           %s",
		data.PacketsSum, len(conData.Connections), utils.BytesToHumanReadable(int64(data.ReceivedBytes)+int64(data.LocalReceivedBytes)+int64(data.ReceivedStartBytes)),
		utils.BytesToHumanReadable(int64(data.TransmitBytes)+int64(data.LocalTransmitBytes)+int64(data.TransmitStartBytes)),
		utils.JsonStringToHumanDuration(data.StartTime),
	)
	graph := charts.NewTree()
	graph.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{Width: "100%", Height: "95vh"}),
		charts.WithTitleOpts(opts.Title{
			Title:    title,
			Subtitle: subtitle,
		}),
		charts.WithTooltipOpts(opts.Tooltip{Show: opts.Bool(true)}),
		charts.WithAnimation(true),
		charts.WithLegendOpts(opts.Legend{Show: opts.Bool(false)}),
	)

	dataPoints := processDataPoints(conData.Connections)

	myTree := generateTreeFromData(data.Namespace, controller.Name, dataPoints)

	graph.AddSeries(title, myTree).
		SetSeriesOptions(
			charts.WithTreeOpts(
				opts.TreeChart{
					Layout:           "orthogonal",
					Orient:           "LR",
					InitialTreeDepth: -1,
					Leaves: &opts.TreeLeaves{
						Label: &opts.Label{Show: opts.Bool(true), Position: "right", Color: "Black"},
					},
				},
			),
			charts.WithLabelOpts(opts.Label{Show: opts.Bool(true), Position: "top", Color: "Black"}),
		)
	return graph
}

type TreeExamples struct{}

func RenderPodNetworkTreePageJson(namespace string, podName string) map[string]interface{} {
	ctrl := kubernetes.ControllerForPod(namespace, podName)
	if ctrl == nil {
		return map[string]interface{}{"error": fmt.Sprintf("could not find controller for pod %s in namespace %s", podName, namespace)}
	}
	stats := dbstats.GetTrafficStatsEntrySumForController(*ctrl, true)
	if stats == nil {
		return map[string]interface{}{"error": fmt.Sprintf("could not find stats for pod %s in namespace %s", podName, namespace)}
	}
	connections := dbstats.GetSocketConnectionsForPod(podName)

	tree := generateTree(*stats, connections, *ctrl)
	page := components.NewPage()
	page.AddCharts(
		tree,
	)

	result := tree.JSON()
	return result
}
func RenderPodNetworkTreePageHtml(namespace string, podName string) string {
	ctrl := kubernetes.ControllerForPod(namespace, podName)
	if ctrl == nil {
		return fmt.Sprintf("could not find controller for pod %s in namespace %s", podName, namespace)
	}
	stats := dbstats.GetTrafficStatsEntrySumForController(*ctrl, true)
	if stats == nil {
		return fmt.Sprintf("could not find stats for pod %s in namespace %s", podName, namespace)
	}
	connections := dbstats.GetSocketConnectionsForPod(podName)

	// TODO: FINALIZE
	ips := connections.UniqueIps()
	mapping := kubernetes.GatherNamesForIps(ips)
	serviceLogger.Info("ip mappings", "mappings", mapping)

	tree := generateTree(*stats, connections, *ctrl)
	page := components.NewPage()
	page.AddCharts(
		tree,
	)

	data := page.RenderContent()
	return string(data)
}
