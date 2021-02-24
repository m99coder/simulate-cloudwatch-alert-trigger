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
	85 \
	3
```

For the above example, it means simulate based on `CPUUtilization` of the `AWS/RDS` instance `my-rds-instance-1` in the time range of 20th Feb, 2021 to 25th Feb, 2021 (exclusive) and apply a threshold of `85` (in this case this represents 85% CPU utilization). If `3` consecutive data points are equal or above the threshold the alarm is treated to be triggered.

```bash
threshold: 85
necessary consecutive hits: 3
start:  2021-02-20 00:00:00 +0100 CET
end:    2021-02-25 00:00:00 +0100 CET

6604 datapoints received
first:  2021-02-20 00:00:00 +0100 CET
last:   2021-02-24 14:03:00 +0100 CET

2021-02-22 16:07:00 +0100 CET:  86 (consecutive hits: 1, triggers: 0, streak: 1)
2021-02-22 16:08:00 +0100 CET:  88 (consecutive hits: 2, triggers: 0, streak: 2)
2021-02-22 16:09:00 +0100 CET:  86 (consecutive hits: 3, triggers: 1, streak: 3)
2021-02-22 16:18:00 +0100 CET:  87 (consecutive hits: 1, triggers: 1, streak: 3)
2021-02-22 16:19:00 +0100 CET:  86 (consecutive hits: 2, triggers: 1, streak: 3)
2021-02-22 16:20:00 +0100 CET:  89 (consecutive hits: 3, triggers: 2, streak: 3)
2021-02-22 16:21:00 +0100 CET:  86 (consecutive hits: 4, triggers: 2, streak: 4)

events: 7, triggers: 2, streak: 4
```

The output shows that within the 6604 datapoints 7 have been above the threshold of 85. All of them are listed as well. It further shows that the alarm would have been triggered 2 times and the longest streak of consecutive data points equal or above the threshold was 4 within the given time range.
