package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"math"
	"mogenius-operator/src/structs"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"
	"time"
)

const HYPERFINE_RUNS = "100"
const HYPERFINE_EXPORT_JSON = "result.json"
const SOCKETAPI_URL = "http://localhost:1337/socketapi"
const PATTERNLOGS_FILE = "/tmp/patternlogs.jsonl"
const SUMMARY_FILE = "_summary.md"

//go:embed advanced_metrics.py
var advancedMetrics string
var advancedMetricsBin string

//go:embed plot_progression.py
var plotProgression string
var plotProgressionBin string

//go:embed plot_whisker.py
var plotWhisker string
var plotWhiskerBin string

// patterns that create/update/delete resources should not be benchmarked
// add those patterns to this list to skip the benchmarking step
var skipPatternBenchmark = []string{
	"create/user",
	"create/grant",
	"create/workspace",
}

type LogLine struct {
	Time     time.Duration    `json:"time"`
	Datagram structs.Datagram `json:"datagram"`
}

func main() {
	err := loadPythonScripts()
	if err != nil {
		panic(err)
	}

	logdir := "patternlogs_" + time.Now().Format(time.RFC3339)
	err = os.MkdirAll(logdir, 0755)
	if err != nil {
		panic(err)
	}

	loglines, err := loadPatternlogs(logdir)
	if err != nil {
		panic(err)
	}

	summaryfile, err := initializeSummary(logdir, loglines)
	if err != nil {
		panic(err)
	}

	err = writePatternCounter(summaryfile, loglines)
	if err != nil {
		panic(err)
	}

	patterns := []string{}
	for _, logline := range loglines {
		if !slices.Contains(patterns, logline.Datagram.Pattern) {
			patterns = append(patterns, logline.Datagram.Pattern)
		}
	}
	slices.Sort(patterns)

	for _, pattern := range patterns {
		fmt.Printf("Pattern: %s\n", pattern)
		if slices.Contains(skipPatternBenchmark, pattern) {
			fmt.Printf("Skipping Benchmark...\n")
			continue
		}
		for idx, logline := range loglines {
			if logline.Datagram.Pattern == pattern {
				err := benchmarkDatagram(logline.Datagram)
				if err != nil {
					panic(err)
				}
				err = evaluateBenchmarkResults(summaryfile, logdir, idx, logline)
				if err != nil {
					panic(err)
				}
			}
		}
	}
}

func initializeSummary(logdir string, loglines []LogLine) (*os.File, error) {
	file, err := os.Create(path.Join(logdir, SUMMARY_FILE))
	if err != nil {
		return nil, err
	}

	headline := fmt.Sprintf("# Summary for %d LogLines\n\n", len(loglines))
	_, err = file.WriteString(headline)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func loadPythonScripts() error {
	tmpdir := os.TempDir()

	advancedMetricsBin = path.Join(tmpdir, "advanced_metrics.py")
	err := os.WriteFile(advancedMetricsBin, []byte(advancedMetrics), 0755)
	if err != nil {
		return err
	}

	plotProgressionBin = path.Join(tmpdir, "plot_progression.py")
	err = os.WriteFile(plotProgressionBin, []byte(plotProgression), 0755)
	if err != nil {
		return err
	}

	plotWhiskerBin = path.Join(tmpdir, "plot_whisker.py")
	err = os.WriteFile(plotWhiskerBin, []byte(plotWhisker), 0755)
	if err != nil {
		return err
	}

	return nil
}

func loadPatternlogs(logdir string) ([]LogLine, error) {
	loglinebytes, err := os.ReadFile(PATTERNLOGS_FILE)
	if err != nil {
		return []LogLine{}, err
	}

	err = os.WriteFile(path.Join(logdir, "_patternlogs.jsonl"), loglinebytes, 0644)
	if err != nil {
		return []LogLine{}, err
	}

	loglines := []LogLine{}

	for data := range bytes.Lines(loglinebytes) {
		var logline LogLine
		err = json.Unmarshal(data, &logline)
		if err != nil {
			fmt.Println(string(data))
			return []LogLine{}, err
		}
		loglines = append(loglines, logline)
	}

	return loglines, nil
}

func writePatternCounter(file *os.File, loglines []LogLine) error {
	patterns := []string{}
	for _, logline := range loglines {
		if !slices.Contains(patterns, logline.Datagram.Pattern) {
			patterns = append(patterns, logline.Datagram.Pattern)
		}
	}
	slices.Sort(patterns)

	patterncount := map[string]int{}
	for _, logline := range loglines {
		count, ok := patterncount[logline.Datagram.Pattern]
		if !ok {
			count = 0
		}
		count += 1
		patterncount[logline.Datagram.Pattern] = count
	}
	type pcount struct {
		pattern string
		count   int
	}
	patterncountarray := []pcount{}
	for pattern, count := range patterncount {
		patterncountarray = append(patterncountarray, pcount{pattern, count})
	}
	slices.SortFunc(patterncountarray, func(a pcount, b pcount) int {
		return b.count - a.count
	})

	_, err := fmt.Fprintf(file, "|   |   |   |   |\n")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(file, "| - | - | - | - |\n")
	if err != nil {
		return err
	}

	for _, p := range patterncountarray {
		var minDuration time.Duration = math.MaxInt64
		var maxDuration time.Duration = 0

		for _, logline := range loglines {
			if logline.Datagram.Pattern == p.pattern {
				if minDuration > logline.Time {
					minDuration = logline.Time
				}
				if maxDuration < logline.Time {
					maxDuration = logline.Time
				}
			}
		}

		_, err := fmt.Fprintf(file, "| __`%s`__ | %d requests | min `%s` | max `%s` |\n", p.pattern, p.count, minDuration, maxDuration)
		if err != nil {
			return err
		}
		for _, logline := range loglines {
			if logline.Datagram.Pattern == p.pattern {
				_, err := fmt.Fprintf(file, "| | ID `%s` | Duration `%.9fs` | |\n", logline.Datagram.Id, logline.Time.Seconds())
				if err != nil {
					return err
				}
			}
		}
	}

	_, err = file.WriteString("\n")
	if err != nil {
		return err
	}

	return nil
}

func benchmarkDatagram(datagram structs.Datagram) error {
	data, err := json.MarshalToString(datagram)
	if err != nil {
		return err
	}

	err = execCmd(
		"hyperfine",
		"--runs", HYPERFINE_RUNS,
		"--export-json", HYPERFINE_EXPORT_JSON,
		`curl -s -X GET '`+SOCKETAPI_URL+`' --data '`+data+`'`,
	)
	if err != nil {
		return err
	}

	return nil
}

func execCmd(name string, arg ...string) error {
	fmt.Printf("+ %s %s\n", name, strings.Join(arg, " "))
	cmd := exec.Command(name, arg...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	err := cmd.Start()
	if err != nil {
		return err
	}
	err = cmd.Wait()
	if err != nil {
		return err
	}
	return nil
}

func evaluateBenchmarkResults(summaryfile *os.File, logdir string, idx int, logline LogLine) error {
	prefix := fmt.Sprintf("%d_%s", idx, logline.Datagram.Id)
	results, err := os.ReadFile(HYPERFINE_EXPORT_JSON)
	if err != nil {
		return fmt.Errorf("failed to read hyperfine results: %w", err)
	}

	datagramExportJson := path.Join(logdir, prefix+"_"+HYPERFINE_EXPORT_JSON)
	err = os.WriteFile(datagramExportJson, results, 0644)
	if err != nil {
		return fmt.Errorf("failed to write datagram export json: %w", err)
	}

	err = os.Remove(HYPERFINE_EXPORT_JSON)
	if err != nil {
		return fmt.Errorf("failed to remove hyperfine export json: %w", err)
	}

	datagramJson, err := json.Marshal(logline.Datagram)
	if err != nil {
		return fmt.Errorf("failed to marshal datagram to json: %w", err)
	}

	summarytext := fmt.Sprintf("## Pattern: __`%s`__ ID: __`%s`__\n\n", logline.Datagram.Pattern, logline.Datagram.Id)

	summarytext = summarytext + "### Datagram\n\n"
	summarytext = summarytext + "```json\n"
	summarytext = summarytext + string(datagramJson) + "\n"
	summarytext = summarytext + "```\n\n"

	cmd := exec.Command(advancedMetricsBin, datagramExportJson)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get advanced metrics: %w", err)
	}
	summarytext = summarytext + "### Advanced Metrics\n\n"
	summarytext = summarytext + "```\n"
	summarytext = summarytext + strings.TrimSpace(string(output)) + "\n"
	summarytext = summarytext + "```\n\n"

	progressionDiagram := prefix + "_progression.jpg"
	progressionDiagramPath := path.Join(logdir, progressionDiagram)
	err = execCmd(plotProgressionBin, datagramExportJson, "--output", progressionDiagramPath)
	if err != nil {
		return fmt.Errorf("failed to generate progression diagram: %w", err)
	}

	whiskerDiagram := prefix + "_whisker.jpg"
	whiskerDiagramPath := path.Join(logdir, whiskerDiagram)
	err = execCmd(plotWhiskerBin, datagramExportJson, "--output", whiskerDiagramPath)
	if err != nil {
		return fmt.Errorf("failed to generate whisker diagram: %w", err)
	}

	summarytext = summarytext + "### Diagrams\n\n"
	summarytext = summarytext + "| Progression Diagram | Whisker Diagram |\n"
	summarytext = summarytext + "| - | - |\n"
	summarytext = summarytext + fmt.Sprintf(`| <img height="500px" src="%s"> | <img height="500px" src="%s"> |`+"\n\n", progressionDiagram, whiskerDiagram)

	_, err = summaryfile.WriteString(summarytext)
	if err != nil {
		return fmt.Errorf("failed to write summary text: %w", err)
	}

	return nil
}
