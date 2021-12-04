package main

import (
	"flag"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	mp "github.com/mackerelio/go-mackerel-plugin"
)

type MacosPowermetricsPlugin struct {
	Prefix string
}

func (m MacosPowermetricsPlugin) GraphDefinition() map[string]mp.Graphs {
	labelPrefix := strings.Title(m.MetricKeyPrefix())

	stat, err := m.FetchMetrics()
	if err != nil {
		return nil
	}

	// CPU frequency metrics
	freq_metrics := []mp.Metrics{}
	for key, _ := range stat {
		re := regexp.MustCompile(`frequency.(cpu[0-9]+)`)
		res := re.FindStringSubmatch(key)
		if res != nil {
			// 	{Name: "frequency.cpu0", Label: "CPU0 Frequency MHz", Diff: false},
			freq_metrics = append(freq_metrics, mp.Metrics{Name: key, Label: fmt.Sprintf("%s Frequency MHz", strings.ToUpper(res[1]))})
		}
	}

	// CPU residency metrics
	res_metrics := []mp.Metrics{}
	for key, _ := range stat {
		re := regexp.MustCompile(`residency.(cpu[0-9]+)`)
		res := re.FindStringSubmatch(key)
		if res != nil {
			// 	{Name: "residency.cpu0", Label: "CPU0 Residency", Diff: false},
			res_metrics = append(res_metrics, mp.Metrics{Name: key, Label: fmt.Sprintf("%s Residency", strings.ToUpper(res[1]))})
		}
	}

	return map[string]mp.Graphs{
		"frequency": {
			Label:   labelPrefix + " CPU Frequency",
			Unit:    mp.UnitInteger,
			Metrics: freq_metrics,
		},
		"residency": {
			Label:   labelPrefix + " CPU Residency %",
			Unit:    mp.UnitPercentage,
			Metrics: res_metrics,
		},
	}
}

func (m MacosPowermetricsPlugin) FetchMetrics() (map[string]float64, error) {
	output, err := exec.Command("powermetrics", "--samplers", "cpu_power", "-i", "1000", "-n", "1").Output()
	if err != nil {
		return nil, fmt.Errorf("Failed to execute powermetrics command: %s", err)
	}
	return parsePowermetrics(string(output))
}

func (m MacosPowermetricsPlugin) MetricKeyPrefix() string {
	if m.Prefix == "" {
		m.Prefix = "powermetrics"
	}
	return m.Prefix
}

func parsePowermetrics(output string) (map[string]float64, error) {
	ret := make(map[string]float64)
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		// frequency
		re := regexp.MustCompile(`^CPU ([0-9]+) frequency: ([0-9]+) MHz$`)
		res := re.FindStringSubmatch(line)
		if res != nil {
			freq, err := strconv.ParseFloat(res[2], 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse frequency: %s", err)
			}
			k := fmt.Sprintf("frequency.cpu%s", res[1])
			ret[k] = freq
		}

		// CPU residency
		re = regexp.MustCompile(`^CPU ([0-9]+) active residency: +([0-9\.]+)%`)
		res = re.FindStringSubmatch(line)
		if res != nil {
			residency, err := strconv.ParseFloat(res[2], 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse CPU residency: %s", err)
			}
			k := fmt.Sprintf("residency.cpu%s", res[1])
			ret[k] = residency
		}

	}
	return ret, nil
}

func main() {
	optPrefix := flag.String("metric-key-prefix", "powermetrics", "Metric key prefix")
	optTempfile := flag.String("tempfile", "", "Temp file name")
	flag.Parse()

	u := MacosPowermetricsPlugin{
		Prefix: *optPrefix,
	}
	plugin := mp.NewMackerelPlugin(u)
	plugin.Tempfile = *optTempfile
	plugin.Run()
}
