package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
)

// MetricDatapoint describes a single datapoint
type MetricDatapoint struct {
	Timestamp time.Time
	Value     float64
}

func main() {
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

	threshold, err := strconv.Atoi(os.Args[7])
	if err != nil {
		fmt.Printf("Cannot parse threshold: %v\n", os.Args[7])
		os.Exit(1)
	}

	necessaryConsecutiveHits, err := strconv.Atoi(os.Args[8])
	if err != nil {
		fmt.Printf("Cannot parse consecutive hits: %v\n", os.Args[8])
		os.Exit(1)
	}

	fmt.Printf("threshold: %d\n", threshold)
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
		events          = 0 // counts number of relevant events
		hits            = 0 // counts hits per trigger
		consecutiveHits = 0 // counts consecutive hits
		triggers        = 0 // counts simulated triggers
		streak          = 0 // counts longest streak of consecutive hits
	)

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

			fmt.Printf(
				"%v: %3d (consecutive hits: %d, triggers: %d, streak: %d)\n",
				metricsData[reverseIndex].Timestamp.In(localLoc),
				int64(metricsData[reverseIndex].Value),
				consecutiveHits,
				triggers,
				streak,
			)

		} else {
			hits = 0
			consecutiveHits = 0
		}
	}

	fmt.Printf("\nevents: %d, triggers: %d, streak: %d\n", events, triggers, streak)
}
