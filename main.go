package main

import (
	"fmt"
	"os"
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
}

// OutputLine describes a single output line that can be a datapoint or a separator
type OutputLine struct {
	Datapoint *OutputDatapoint
}

func getMaxLength(lines []OutputLine, accessor func(line OutputLine) string) int {
	maxLength := 0
	for _, line := range lines {
		length := len(accessor(line))
		if length > maxLength {
			maxLength = length
		}
	}
	return maxLength
}

func getFormatString(largestValueLength int, largestDiffLength int, separator string) string {
	return "%-30v" + separator +
		"%-" + fmt.Sprintf("%d", largestValueLength+2) + "v" + separator +
		"%-" + fmt.Sprintf("%d", largestDiffLength+2) + "v" + separator +
		"%-6v" + separator +
		"%-6v" + separator +
		"%-6v" + separator +
		"%s"
}

func main() {

	var (
		colorReset  = "\033[0m"
		colorRed    = "\033[31m"
		colorYellow = "\033[33m"
		colorWhite  = "\033[37m"
	)

	if len(os.Args) < 9 {
		fmt.Println("Missing arguments")
		fmt.Printf("%s\n\t[namespace]\n\t[metricName]\n\t[dimensionName]\n\t[dimensionValue]\n\t[startTime]\n\t[endTime]\n\t[threshold]\n\t[consecutiveHits]\n\n", os.Args[0])
		fmt.Printf("Example:\n%s \\\n\tAWS/RDS \\\n\tCPUUtilization \\\n\tDBInstanceIdentifier \\\n\tmy-rds-instance-1 \\\n\t2021-02-20T00:00:00+01:00 \\\n\t2021-02-25T00:00:00+01:00 \\\n\t85 \\\n\t3\n", os.Args[0])
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

	fmt.Printf("threshold: %.2f\n", threshold)
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
	fmt.Printf("last:   %v\n\n", metricsData[0].Timestamp.In(localLoc))

	var (
		events          int = 0 // counts number of relevant events
		hits            int = 0 // counts hits per trigger
		consecutiveHits int = 0 // counts consecutive hits
		triggers        int = 0 // counts simulated triggers
		streak          int = 0 // counts longest streak of consecutive hits
	)

	// collect output lines
	outputs := []OutputLine{}

	for index := range metricsData {
		reverseIndex := len(metricsData) - index - 1

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
			if consecutiveHits >= necessaryConsecutiveHits {
				blockColor = string(colorRed)
			}
			blocks := strings.Repeat("◼", int(consecutiveHits))

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
				},
			})

		} else {
			hits = 0
			consecutiveHits = 0
		}
	}

	// OUTPUT ---
	largestValueLength := getMaxLength(outputs, func(line OutputLine) string {
		if line.Datapoint == nil {
			return ""
		}
		return fmt.Sprintf("%.2f", line.Datapoint.Value)
	})
	largestDiffLength := getMaxLength(outputs, func(line OutputLine) string {
		if line.Datapoint == nil {
			return ""
		}
		return fmt.Sprintf("%.2f", line.Datapoint.Diff)
	})

	fmt.Printf(
		getFormatString(largestValueLength, largestDiffLength, "│")+"\n",
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
				getFormatString(largestValueLength, largestDiffLength, "┼")+"\n",
				strings.Repeat("─", 30),
				strings.Repeat("─", largestValueLength+2),
				strings.Repeat("─", largestDiffLength+2),
				strings.Repeat("─", 6),
				strings.Repeat("─", 6),
				strings.Repeat("─", 6),
				strings.Repeat("─", 20),
			)
		} else {
			fmt.Printf(
				getFormatString(largestValueLength, largestDiffLength, "│")+"\n",
				line.Datapoint.Timestamp,
				" "+fmt.Sprintf("%.2f", line.Datapoint.Value),
				" "+fmt.Sprintf("%.2f", line.Datapoint.Diff),
				" "+fmt.Sprintf("%d", line.Datapoint.ConsecutiveHits),
				" "+fmt.Sprintf("%d", line.Datapoint.TriggerCount),
				" "+fmt.Sprintf("%d", line.Datapoint.LongestStreak),
				" "+line.Datapoint.StreakVisualization,
			)
		}
	}
	fmt.Printf(
		getFormatString(largestValueLength, largestDiffLength, "┴")+"\n",
		strings.Repeat("─", 30),
		strings.Repeat("─", largestValueLength+2),
		strings.Repeat("─", largestDiffLength+2),
		strings.Repeat("─", 6),
		strings.Repeat("─", 6),
		strings.Repeat("─", 6),
		strings.Repeat("─", 20),
	)
	fmt.Printf("CH: consecutive hits, TC: trigger count, LS: longest streak\n\n")

	fmt.Printf(
		"triggers: %s, streak: %s, events: %s\n",
		string(colorWhite)+fmt.Sprint(triggers)+string(colorReset),
		string(colorWhite)+fmt.Sprint(streak)+string(colorReset),
		string(colorWhite)+fmt.Sprint(events)+string(colorReset),
	)
}
