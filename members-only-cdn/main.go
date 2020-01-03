package main

import (
	"errors"
	"fmt"
  "os"
  "strings"
  "time"
  "sync"

  "crypto/sha256"
  "encoding/hex"
  "encoding/json"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
  //"github.com/aws/aws-sdk-go/aws/awserr"
  "github.com/aws/aws-sdk-go/service/s3"

  "github.com/k0kubun/pp"

  "github.com/dogwood008/members_only_cdn/cwlogs"
  "github.com/dogwood008/members_only_cdn/authorization"
)

var (
	// ErrNoIP No IP found in response
	ErrNoIP = errors.New("No IP in HTTP response")

	// ErrNon200Response non 200 status code in response
	ErrNon200Response = errors.New("Non 200 Response found")

	ErrInvalidHash = errors.New("Given Auth Token is Invalid")
	ErrNoUserHashs = errors.New("UserID Hash Map is Empty")

  EnvMapJsonString  = os.Getenv("USER_TOKEN_MAP_JSON")
  EnvS3BucketName   = os.Getenv("BUCKET_NAME")
  EnvLogGroupName   = os.Getenv("CLOUD_WATCH_LOG_GROUP_NAME")
  EnvCloudWatchSetup = getEnv("CLOUD_WATCH_ENABLE_SETUP", "false") == "true"

  EnvAWSRegion = os.Getenv("AWS_REGION")
  EnvAWSAccessKeyId = os.Getenv("AWS_ACCESS_KEY_ID")
  EnvAWSSecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")

  AwsSession = session.New()
  AwsConfig  = aws.NewConfig().WithRegion(EnvAWSRegion)

  cloudWatchLogs = cwlogs.CWLogs {Setup: EnvCloudWatchSetup, LogGroupName: &EnvLogGroupName}
)

// https://stackoverflow.com/questions/40326540/how-to-assign-default-value-if-env-var-is-empty
func getEnv(key, fallback string) string {
    if value, ok := os.LookupEnv(key); ok {
        return value
    }
    return fallback
}

func userId(hash string, jsonString string) (string, error) {
  if jsonString == "" {
    return "", ErrNoUserHashs
  }
  var extractedUserId string
  var intf interface{}
  bytes := []byte(jsonString)
  json.Unmarshal(bytes, &intf)
  hmm := intf.(map[string]interface{})
  hmmm :=hmm["Maps"].(map[string]interface{})

  pp.Print(hash)
  uncastUid := hmmm[hash]
  if uncastUid == nil {
    return "", ErrInvalidHash
  }
  extractedUserId = uncastUid.(string)
  return extractedUserId, nil
}

func auth(authHeader string) (string, error) {
  var hexToken, authRawToken string
  authRawToken = strings.Replace(authHeader, "Bearer ", "", 1)
  bytes := sha256.Sum256([]byte(authRawToken))
  hexToken = hex.EncodeToString(bytes[:])
  uid, err := userId(hexToken, EnvMapJsonString)
  return uid, err
}

func s3UrlWithPreSign (keyName string, bucketName string, region string) (string, error) {
  // https://qiita.com/sakayuka/items/1328c1ad93f9b982a0d5
  svc := s3.New(AwsSession, AwsConfig)

  req, _ := svc.GetObjectRequest(&s3.GetObjectInput{
    Bucket: aws.String(bucketName),
    Key:    aws.String(keyName),
  })
  url, err := req.Presign(time.Minute * 10)
  return url, err
}

func outputLog2CloudWatch (userId string, s3Key string, err string) {
  log := fmt.Sprintf(",%s,\"s3://%s%s\",\"%s\"", userId, EnvS3BucketName, s3Key, err)
  cloudWatchLogs.OutputLog2CloudWatch(&log)
}

func checkPermittedFileId (ch chan<- bool, waitGroup *sync.WaitGroup, projectId string, objectId string, userId string, requestedFileId string) {
  isOkToAllow := authorization.Authorize(projectId, objectId, userId, requestedFileId)
  ch <- isOkToAllow
  waitGroup.Done()
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
  // Path: /projects/{project_id}/objects/{object_id}/users/{user_id}/files/{id_full}
  params := request.PathParameters
  projectId := params["project_id"]
  objectId := params["object_id"]
  userIdInPath := params["user_id"]
  fileId := params["file_id"]
  s3Key := fmt.Sprintf("/%s/%s/%s", projectId, objectId, fileId)

  waitGroup := &sync.WaitGroup{}
  waitGroup.Add(1)
  checkPermissionCh := make(chan bool, 1)
  defer close(checkPermissionCh)  // https://qiita.com/convto/items/b2e95e549f35a1beb0b8
  go checkPermittedFileId(checkPermissionCh, waitGroup, projectId, objectId, userIdInPath, fileId)

  authHeader := request.Headers["Authorization"]
  userIdInAuthHeader, err := auth(authHeader)
  if userIdInAuthHeader != userIdInPath {
    err = ErrInvalidHash
  }
  if err != nil {
    var code int
    var body string
    switch err {
      case ErrNoUserHashs: code = 500; body = "Server setup does not finished. (Error code: 001)"
      case ErrInvalidHash: code = 403; body = "Invalid auth token given. (Error code: 002)"
    }
    log := fmt.Sprintf("userIdInPath:%s/authHeader:%s", userIdInPath, authHeader)
    outputLog2CloudWatch(userIdInPath, s3Key, log)
    return events.APIGatewayProxyResponse{
      Body      : body,
      StatusCode: code,
    }, nil
  }

  presignedUrl, err := s3UrlWithPreSign(s3Key, EnvS3BucketName, EnvAWSRegion)
  if err != nil {
    body := "Internal server error (Error code: 003)"
    outputLog2CloudWatch(userIdInPath, s3Key, body)
    return events.APIGatewayProxyResponse{
      Body      : body,
      StatusCode: 500,
    }, nil
  }

  waitGroup.Wait()
  isOkToAllow := <-checkPermissionCh
  if !isOkToAllow {
    body := "The requested file id is invalid for you. (Error code: 004)"
    outputLog2CloudWatch(userIdInPath, s3Key, body)
    return events.APIGatewayProxyResponse{
      Body      : body,
      StatusCode: 403,
    }, nil
  }

  outputLog2CloudWatch(userIdInPath, s3Key, "succeeded")
	return events.APIGatewayProxyResponse{
    Body      : presignedUrl,
		StatusCode: 200,
	}, nil
}

func keys(m map[string]string) []string {
    ks := []string{}
    for k, _ := range m {
        ks = append(ks, k)
    }
    return ks
}

func main() {
	lambda.Start(handler)
}
