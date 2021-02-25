package main

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
)

// MetricDatapoint describes a single metric datapoint
type MetricDatapoint struct {
	Timestamp time.Time
	Value     float64
}

// OutputDatapoint describes a single output datapoint
type OutputDatapoint struct {
	Timestamp           time.Time
	Value               float64
	Diff                float64
	ConsecutiveHits     int
	TriggerCount        int
	LongestStreak       int
	StreakVisualization string
	StreakBlocks        string
}

// OutputLine describes a single output line that can be a datapoint or a separator
type OutputLine struct {
	Datapoint *OutputDatapoint
}

func getMaxLength(lines []OutputLine, accessor func(line OutputLine) string) int {
	maxLength := 0
	for _, line := range lines {
		// unicode characters return more than just 1 to len
		// this is equivalent to `for r := range s`
		length := len([]rune(accessor(line)))
		if length > maxLength {
			maxLength = length
		}
	}
	return maxLength
}

func getFormatString(largestValueLength int, largestDiffLength int, largestStreakLength int, separator string) string {
	return "%-30v" + separator +
		"%-" + fmt.Sprintf("%d", largestValueLength+2) + "v" + separator +
		"%-" + fmt.Sprintf("%d", largestDiffLength+2) + "v" + separator +
		"%-6v" + separator +
		"%-6v" + separator +
		"%-6v" + separator +
		"%-" + fmt.Sprintf("%d", largestStreakLength+2) + "v"
}

func main() {

	var (
		colorReset  = "\033[0m"
		colorRed    = "\033[31m"
		colorYellow = "\033[33m"
		colorWhite  = "\033[37m"
	)

	precision := "%.2f"

	if len(os.Args) < 9 {
		fmt.Println("Missing arguments")
		fmt.Printf(
			"%s\n"+
				"\t[namespace]\n"+
				"\t[metricName]\n"+
				"\t[dimensionName]\n"+
				"\t[dimensionValue]\n"+
				"\t[startTime]\n"+
				"\t[endTime]\n"+
				"\t[threshold]\n"+
				"\t[consecutiveHits]\n\n", os.Args[0])
		fmt.Printf(
			"Example:\n"+
				"%s \\\n"+
				"\tAWS/RDS \\\n"+
				"\tCPUUtilization \\\n"+
				"\tDBInstanceIdentifier \\\n"+
				"\tmy-rds-instance-1 \\\n"+
				"\t2021-02-20T00:00:00+01:00 \\\n"+
				"\t2021-02-25T00:00:00+01:00 \\\n"+
				"\t85.00 \\\n"+
				"\t3\n", os.Args[0])
		os.Exit(1)
	}

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	svc := cloudwatch.New(sess)

	namespace := os.Args[1]
	metricName := os.Args[2]

	// note that `dimensions` can contain a maximum of 10 dimensions
	dimensions := []*cloudwatch.Dimension{
		{
			Name:  aws.String(os.Args[3]),
			Value: aws.String(os.Args[4]),
		},
	}

	// note that `endTime` is exclusive
	// note that a maximum of 14 days per batch is available
	startTime, _ := time.Parse(time.RFC3339, os.Args[5])
	endTime, _ := time.Parse(time.RFC3339, os.Args[6])

	threshold, err := strconv.ParseFloat(os.Args[7], 64)
	if err != nil {
		fmt.Printf("Cannot parse threshold: %v\n", os.Args[7])
		os.Exit(1)
	}

	necessaryConsecutiveHits, err := strconv.Atoi(os.Args[8])
	if err != nil {
		fmt.Printf("Cannot parse consecutive hits: %v\n", os.Args[8])
		os.Exit(1)
	}

	fmt.Printf("threshold: "+precision+"\n", threshold)
	fmt.Printf("necessary consecutive hits: %d\n", necessaryConsecutiveHits)
	fmt.Printf("start:  %v\n", startTime)
	fmt.Printf("end:    %v\n\n", endTime)

	input := &cloudwatch.GetMetricDataInput{
		MetricDataQueries: []*cloudwatch.MetricDataQuery{
			{
				Id: aws.String("query" + metricName),
				MetricStat: &cloudwatch.MetricStat{
					Metric: &cloudwatch.Metric{
						Namespace:  aws.String(namespace),
						MetricName: aws.String(metricName),
						Dimensions: dimensions,
					},
					Period: aws.Int64(60),
					Stat:   aws.String("Average"),
				},
			},
		},
		StartTime: &startTime,
		EndTime:   &endTime,
	}

	localLoc, _ := time.LoadLocation("Local")

	metricsData := []MetricDatapoint{}

	err = svc.GetMetricDataPages(input,
		func(page *cloudwatch.GetMetricDataOutput, lastPage bool) bool {
			for _, result := range page.MetricDataResults {
				for index, timestamp := range result.Timestamps {
					metricsData = append(metricsData, MetricDatapoint{
						Timestamp: *timestamp,
						Value:     *(result.Values[index]),
					})
				}
			}
			return true
		})
	if err != nil {
		fmt.Println("Got error getting metrics data:")
		fmt.Println(err.Error())
		os.Exit(1)
	}

	fmt.Printf("%d datapoints received\n", len(metricsData))
	fmt.Printf("first:  %v\n", metricsData[len(metricsData)-1].Timestamp.In(localLoc))
	fmt.Printf("last:   %v\n", metricsData[0].Timestamp.In(localLoc))

	// ANALYSIS ---

	var (
		events          int = 0 // counts number of relevant events
		hits            int = 0 // counts hits per trigger
		consecutiveHits int = 0 // counts consecutive hits
		triggers        int = 0 // counts simulated triggers
		streak          int = 0 // counts longest streak of consecutive hits
	)

	// collect output lines
	outputs := []OutputLine{}

	// statistics
	var (
		min    *float64  = nil
		max    *float64  = nil
		num    int64     = 0
		sum    float64   = 0
		values []float64 = []float64{}
	)

	for index := range metricsData {
		reverseIndex := len(metricsData) - index - 1

		if min == nil || *min > metricsData[reverseIndex].Value {
			min = &metricsData[reverseIndex].Value
		}
		if max == nil || *max < metricsData[reverseIndex].Value {
			max = &metricsData[reverseIndex].Value
		}
		num++
		sum += metricsData[reverseIndex].Value
		values = append(values, metricsData[reverseIndex].Value)

		if metricsData[reverseIndex].Value >= float64(threshold) {
			events++
			hits++
			consecutiveHits++

			// we are on a streak
			if consecutiveHits > streak {
				streak = consecutiveHits
			}

			// if hits is equal to consecutiveHits (meaning first loop)
			// we reached the necessary consecutive hits to trigger
			if hits == consecutiveHits && hits >= necessaryConsecutiveHits {
				triggers++
				hits = 0
			}

			blockColor := string(colorYellow)
			blocks := strings.Repeat("◼", int(consecutiveHits))
			if consecutiveHits >= necessaryConsecutiveHits {
				blockColor = string(colorRed)
				blocks = strings.Repeat("◼", int(necessaryConsecutiveHits))
				if consecutiveHits > necessaryConsecutiveHits {
					blocks += "+"
				}
			}

			if consecutiveHits == 1 {
				outputs = append(outputs, OutputLine{
					Datapoint: nil,
				})
			}

			outputs = append(outputs, OutputLine{
				Datapoint: &OutputDatapoint{
					Timestamp:           metricsData[reverseIndex].Timestamp.In(localLoc),
					Value:               metricsData[reverseIndex].Value,
					Diff:                metricsData[reverseIndex].Value - threshold,
					ConsecutiveHits:     consecutiveHits,
					TriggerCount:        triggers,
					LongestStreak:       streak,
					StreakVisualization: blockColor + blocks + string(colorReset),
					StreakBlocks:        blocks,
				},
			})

		} else {
			hits = 0
			consecutiveHits = 0
		}
	}

	// OUTPUT ---

	// sort values in ascending order (for median and percentile)
	sort.Float64s(values)

	median := float64(0)
	if num%2 == 0 {
		median = 0.5 * (values[num/2] + values[num/2+1])
	} else {
		median = values[(num+1)/2]
	}

	p9900 := values[int64(math.RoundToEven(float64(num)*0.99))]
	p9990 := values[int64(math.RoundToEven(float64(num)*0.999))]
	p9999 := values[int64(math.RoundToEven(float64(num)*0.9999))]

	fmt.Printf(
		"min: %s, max: %s, mean: %s, median: %s\n",
		string(colorWhite)+fmt.Sprintf(precision, *min)+string(colorReset),
		string(colorWhite)+fmt.Sprintf(precision, *max)+string(colorReset),
		string(colorWhite)+fmt.Sprintf(precision, sum/float64(num))+string(colorReset),
		string(colorWhite)+fmt.Sprintf(precision, median)+string(colorReset),
	)
	fmt.Printf(
		"p99: %s, p99.9: %s, p99.99: %s\n\n",
		string(colorWhite)+fmt.Sprintf(precision, p9900)+string(colorReset),
		string(colorWhite)+fmt.Sprintf(precision, p9990)+string(colorReset),
		string(colorWhite)+fmt.Sprintf(precision, p9999)+string(colorReset),
	)

	var (
		largestValueLength  = 0
		largestDiffLength   = 0
		largestStreakLength = 6 // length of “Streak” as column title
	)

	for _, line := range outputs {
		if line.Datapoint != nil {
			valueLength := len([]rune(fmt.Sprintf(precision, line.Datapoint.Value)))
			if valueLength > largestValueLength {
				largestValueLength = valueLength
			}

			diffLength := len([]rune(fmt.Sprintf(precision, line.Datapoint.Diff)))
			if diffLength > largestDiffLength {
				largestDiffLength = diffLength
			}

			streakLength := len([]rune(fmt.Sprintf("%s", line.Datapoint.StreakBlocks)))
			if streakLength > largestStreakLength {
				largestStreakLength = streakLength
			}

			if min == nil || *min > line.Datapoint.Value {
				min = &line.Datapoint.Value
			}
			if max == nil || *max < line.Datapoint.Value {
				max = &line.Datapoint.Value
			}
		}
	}

	fmt.Printf(
		getFormatString(largestValueLength, largestDiffLength, largestStreakLength, "│")+"\n",
		"Timestamp",
		" Value",
		" Diff",
		" CH",
		" TC",
		" LS",
		" Streak",
	)
	for _, line := range outputs {
		if line.Datapoint == nil {
			fmt.Printf(
				getFormatString(largestValueLength, largestDiffLength, largestStreakLength, "┼")+"\n",
				strings.Repeat("─", 30),
				strings.Repeat("─", largestValueLength+2),
				strings.Repeat("─", largestDiffLength+2),
				strings.Repeat("─", 6),
				strings.Repeat("─", 6),
				strings.Repeat("─", 6),
				strings.Repeat("─", largestStreakLength+2),
			)
		} else {
			fmt.Printf(
				getFormatString(largestValueLength, largestDiffLength, largestStreakLength, "│")+"\n",
				line.Datapoint.Timestamp,
				" "+fmt.Sprintf(precision, line.Datapoint.Value),
				" "+fmt.Sprintf(precision, line.Datapoint.Diff),
				" "+fmt.Sprintf("%d", line.Datapoint.ConsecutiveHits),
				" "+fmt.Sprintf("%d", line.Datapoint.TriggerCount),
				" "+fmt.Sprintf("%d", line.Datapoint.LongestStreak),
				" "+line.Datapoint.StreakVisualization,
			)
		}
	}
	fmt.Printf(
		getFormatString(largestValueLength, largestDiffLength, largestStreakLength, "┴")+"\n",
		strings.Repeat("─", 30),
		strings.Repeat("─", largestValueLength+2),
		strings.Repeat("─", largestDiffLength+2),
		strings.Repeat("─", 6),
		strings.Repeat("─", 6),
		strings.Repeat("─", 6),
		strings.Repeat("─", largestStreakLength+2),
	)
	fmt.Printf("CH: consecutive hits, TC: trigger count, LS: longest streak\n\n")

	fmt.Printf(
		"triggers: %s, streak: %s, events: %s\n",
		string(colorWhite)+fmt.Sprint(triggers)+string(colorReset),
		string(colorWhite)+fmt.Sprint(streak)+string(colorReset),
		string(colorWhite)+fmt.Sprint(events)+string(colorReset),
	)
}
