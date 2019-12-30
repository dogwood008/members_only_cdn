package cwlogs

import (
  "os"
  "time"
  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/cloudwatchlogs"
)

type CWLogs struct {
  Setup bool
  LogGroupName *string
}

var (
  envAWSRegion = os.Getenv("AWS_REGION")

  awsSession = session.New()
  awsConfig  = aws.NewConfig().WithRegion(envAWSRegion)
  svc = cloudwatchlogs.New(awsSession, awsConfig)
)

// https://dev.classmethod.jp/cloud/aws/put-cloudwatchlogs/
// https://docs.aws.amazon.com/sdk-for-go/api/service/cloudwatchlogs/#CloudWatchLogs.DescribeLogStreams
// https://docs.aws.amazon.com/sdk-for-go/api/service/cloudwatchlogs/#DescribeLogStreamsInput
// https://docs.aws.amazon.com/sdk-for-go/api/service/cloudwatchlogs/#CloudWatchLogs
// https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_DescribeLogStreams.html
// https://docs.aws.amazon.com/sdk-for-go/api/service/cloudwatchlogs/#DescribeLogStreamsOutput
// https://docs.aws.amazon.com/sdk-for-go/api/service/cloudwatchlogs/#LogStream
func (cwl *CWLogs) nextCloudWatchSequenceToken (logStreamNamePrefix *string) string {
  input := cloudwatchlogs.DescribeLogStreamsInput {LogGroupName: cwl.LogGroupName, LogStreamNamePrefix: logStreamNamePrefix}
  resp, err := svc.DescribeLogStreams(&input)
  if err != nil || len(resp.LogStreams) == 0 {
    return ""
  }
  nextToken := *(resp.LogStreams[0].UploadSequenceToken)
  return nextToken
}

// https://docs.aws.amazon.com/sdk-for-go/api/service/cloudwatchlogs/#CloudWatchLogs.CreateLogGroup
// https://docs.aws.amazon.com/sdk-for-go/api/service/cloudwatchlogs/#CreateLogGroupInput
func (cwl *CWLogs) createLogGroup () {
  input := cloudwatchlogs.CreateLogGroupInput{LogGroupName: cwl.LogGroupName}
  svc.CreateLogGroup(&input)
}

// https://docs.aws.amazon.com/sdk-for-go/api/service/cloudwatchlogs/#CloudWatchLogs.CreateLogStream
// https://docs.aws.amazon.com/sdk-for-go/api/service/cloudwatchlogs/#CreateLogStreamInput
func (cwl *CWLogs) createLogStream (logStreamName *string) {
  input := cloudwatchlogs.CreateLogStreamInput {LogGroupName: cwl.LogGroupName, LogStreamName: logStreamName}
  svc.CreateLogStream(&input)
}

// https://docs.aws.amazon.com/sdk-for-go/api/service/cloudwatchlogs/#CloudWatchLogs.PutLogEvents
// https://docs.aws.amazon.com/sdk-for-go/api/service/cloudwatchlogs/#PutLogEventsInput
// https://docs.aws.amazon.com/sdk-for-go/api/service/cloudwatchlogs/#InputLogEvent
// https://confrage.jp/go-%E8%A8%80%E8%AA%9E%E3%81%AEtime-%E3%83%91%E3%83%83%E3%82%B1%E3%83%BC%E3%82%B8%E3%81%8B%E3%82%89%E3%83%9F%E3%83%AA%E7%A7%92%E3%82%92%E6%B1%82%E3%82%81%E3%82%8B%E6%96%B9%E6%B3%95/
func (cwl *CWLogs) OutputLog2CloudWatch (logBody *string) (*cloudwatchlogs.PutLogEventsOutput, error){
  unixTimeInMil := time.Now().UnixNano() / int64(time.Millisecond)
  ile := cloudwatchlogs.InputLogEvent{Message: logBody, Timestamp: &unixTimeInMil}
  iles := []*cloudwatchlogs.InputLogEvent{&ile}

  t := time.Now()
  logStreamName := t.Format("20060102Z")

  if cwl.Setup {
    cwl.createLogGroup()
    cwl.createLogStream(&logStreamName)
  }

  nextCloudWatchSequenceToken := cwl.nextCloudWatchSequenceToken(&logStreamName)
  plei := cloudwatchlogs.PutLogEventsInput{
    LogEvents: iles,
    LogGroupName: cwl.LogGroupName,
    LogStreamName: &logStreamName}
  if (nextCloudWatchSequenceToken != "") {
    plei.SetSequenceToken(nextCloudWatchSequenceToken)
  }
  resp, err := svc.PutLogEvents(&plei)
  return resp, err
}

/* for debug
func main() {
  logGroupName := "members-only-cdn"
  cwlogs := CWLogs {LogGroupName: &logGroupName, Setup: false}
  cwlogs.OutputLog2CloudWatch("userId", "index.html", "dogwood008-members-only-cdn")
}
*/
