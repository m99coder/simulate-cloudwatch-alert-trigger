# simulate-cloudwatch-alert-trigger

This tool simply pulls AWS CloudWatch Metrics Data and simulates alert triggers based on a given dimension for a given AWS resource. As a prerequisite the tool requires to be logged in into an appropriate AWS account (e.g. calling `aws login` upfront or having valid AWS credentials in the `~/.aws/` folder).

```bash
$ ./simulate-cloudwatch-alert-trigger
Missing arguments
./simulate-cloudwatch-alert-trigger
	[namespace]
	[metricName]
	[dimensionName]
	[dimensionValue]
	[startTime]
	[endTime]
	[threshold]
	[consecutiveHits]

Example:
./simulate-cloudwatch-alert-trigger \
	AWS/RDS \
	CPUUtilization \
	DBInstanceIdentifier \
	my-rds-instance-1 \
	2021-02-20T00:00:00+01:00 \
	2021-02-25T00:00:00+01:00 \
	85.00 \
	3
```

For the above example, it means simulate based on `CPUUtilization` of the `AWS/RDS` instance `my-rds-instance-1` in the time range of 20th Feb, 2021 to 25th Feb, 2021 (exclusive) and apply a threshold of `85.00` (in this case this represents 85% CPU utilization). If `3` consecutive data points are equal or above the threshold the alarm is treated to be triggered.

The sampling rate is statically set to 60 seconds and the metric is calculated based on `Average`, which usually makes a lot of sense as otherwise only snapshot values or a continuous value would be taken into account.

```bash
threshold: 85.00
necessary consecutive hits: 3
start:  2021-02-20 00:00:00 +0100 CET
end:    2021-02-25 00:00:00 +0100 CET

7200 datapoints received
first:  2021-02-20 00:00:00 +0100 CET
last:   2021-02-24 23:59:00 +0100 CET
min: 2.00, max: 89.00, mean: 4.67, median: 3.00
p99: 22.00, p99.9: 86.00, p99.99: 89.00

Timestamp                     │ Value │ Diff │ CH   │ TC   │ LS   │ Streak
──────────────────────────────┼───────┼──────┼──────┼──────┼──────┼────────
2021-02-22 16:07:00 +0100 CET │ 86.00 │ 1.00 │ 1    │ 0    │ 1    │ ◼
2021-02-22 16:08:00 +0100 CET │ 88.00 │ 3.00 │ 2    │ 0    │ 2    │ ◼◼
2021-02-22 16:09:00 +0100 CET │ 86.00 │ 1.00 │ 3    │ 1    │ 3    │ ◼◼◼
──────────────────────────────┼───────┼──────┼──────┼──────┼──────┼────────
2021-02-22 16:18:00 +0100 CET │ 87.00 │ 2.00 │ 1    │ 1    │ 3    │ ◼
2021-02-22 16:19:00 +0100 CET │ 86.00 │ 1.00 │ 2    │ 1    │ 3    │ ◼◼
2021-02-22 16:20:00 +0100 CET │ 89.00 │ 4.00 │ 3    │ 2    │ 3    │ ◼◼◼
2021-02-22 16:21:00 +0100 CET │ 86.00 │ 1.00 │ 4    │ 2    │ 4    │ ◼◼◼+
──────────────────────────────┴───────┴──────┴──────┴──────┴──────┴────────
CH: consecutive hits, TC: trigger count, LS: longest streak

triggers: 2, streak: 4, events: 7
```

The output shows that within the 7200 datapoints 7 have been above the threshold of 85.00. All of them are listed as well. It further shows that the alarm would have been triggered 2 times and the longest streak of consecutive data points equal or above the threshold was 4 within the given time range.

For statistical analysis minimum, maximum, mean, and median for the whole dataset are calculated. As well as the percentiles 99, 99.9, and 99.99.
