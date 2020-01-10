package main

import (
  "errors"
  "fmt"
  "os"
  "strings"
  "time"
  "sync"
  "mime"

  "path/filepath"
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
  errInvalidHash = errors.New("Given Auth Token is Invalid")
  errNoUserHashs = errors.New("UserID Hash Map is Empty")
  errInvalidDlOrUl = errors.New("Given was not \"UL\" or \"DB\"")

  envMapJSONString  = os.Getenv("USER_TOKEN_MAP_JSON")
  envS3ULBucketName   = os.Getenv("UL_BUCKET_NAME")
  envS3DLBucketName   = os.Getenv("DL_BUCKET_NAME")
  envLogGroupName   = os.Getenv("CLOUD_WATCH_LOG_GROUP_NAME")
  envCloudWatchSetup = getEnv("CLOUD_WATCH_ENABLE_SETUP", "false") == "true"

  envAWSRegion = os.Getenv("AWS_REGION")

  awsSession = session.New()
  awsConfig  = aws.NewConfig().WithRegion(envAWSRegion)

  cloudWatchLogs = cwlogs.CWLogs {Setup: envCloudWatchSetup, LogGroupName: &envLogGroupName}
)

type params struct {
  ProjectID    string
  ObjectID     string
  UserIDInPath string
  FileID       string
}


// https://stackoverflow.com/questions/40326540/how-to-assign-default-value-if-env-var-is-empty
func getEnv(key, fallback string) string {
    if value, ok := os.LookupEnv(key); ok {
        return value
    }
    return fallback
}

func userID(hash string, jsonString string) (string, error) {
  if jsonString == "" {
    return "", errNoUserHashs
  }
  var extractedUserID string
  var intf interface{}
  bytes := []byte(jsonString)
  json.Unmarshal(bytes, &intf)
  hmm := intf.(map[string]interface{})
  hmmm :=hmm["Maps"].(map[string]interface{})

  uncastUID := hmmm[hash]
  if uncastUID == nil {
    return "", errInvalidHash
  }
  extractedUserID = uncastUID.(string)
  return extractedUserID, nil
}

func auth(authHeader string) (string, error) {
  var hexToken, authRawToken string
  authRawToken = strings.Replace(authHeader, "Bearer ", "", 1)
  bytes := sha256.Sum256([]byte(authRawToken))
  hexToken = hex.EncodeToString(bytes[:])
  uid, err := userID(hexToken, envMapJSONString)
  return uid, err
}

func s3GetURLWithPreSign (keyName string, bucketName string, region string) (string, error) {
  // https://qiita.com/sakayuka/items/1328c1ad93f9b982a0d5
  svc := s3.New(awsSession, awsConfig)
  req, _ := svc.GetObjectRequest(&s3.GetObjectInput{
    Bucket: aws.String(bucketName),
    Key:    aws.String(keyName),
  })
  url, err := req.Presign(time.Minute * 10)
  pp.Print(err)
  return url, err
}

func fileType (keyName string) (string) {
  ext := filepath.Ext(strings.Replace(keyName, "/upload", "", 1))
  mime.AddExtensionType(".csv", "text/csv")
  mime.AddExtensionType(".tsv", "text/tab-separated-values")
  mime.AddExtensionType(".txt", "text/plain")
  return mime.TypeByExtension(ext)
}

func s3URLWithPreSign (dlOrUl string, keyName string, bucketName string, region string) (string, error) {
  switch dlOrUl {
    case "DL": return s3GetURLWithPreSign(keyName, bucketName, region)
    case "UL": return s3PutURLWithPreSign(keyName, bucketName, region)
  }
  return "", errInvalidDlOrUl
}

// https://docs.aws.amazon.com/sdk-for-go/api/service/s3/#PutObjectInput
// https://docs.aws.amazon.com/sdk-for-go/api/service/s3/#S3.PutObjectRequest
// https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#CannedACL
// https://www.whatsajunting.com/posts/s3-presigned/
func s3PutURLWithPreSign (keyName string, bucketName string, region string) (string, error) {
  svc := s3.New(awsSession, awsConfig)
  input := s3.PutObjectInput{
    Bucket:      aws.String(bucketName),
    Key:         aws.String(keyName),
    // ContentType: aws.String(fileType(keyName)),
    // ACL:         aws.String("private"),
  }
  req, _ := svc.PutObjectRequest(&input)
  url, err := req.Presign(time.Minute * 10)
  pp.Print(err)
  return url, err
}

func outputLog2CloudWatch (userID string, s3Key string, bucketName string, err string) {
  log := fmt.Sprintf(",%s,\"s3://%s%s\",\"%s\"", userID, bucketName, s3Key, err)
  cloudWatchLogs.OutputLog2CloudWatch(&log)
}

func checkPermittedFileID (ch chan<- bool, waitGroup *sync.WaitGroup, params *params) {
  isOkToAllow := authorization.Authorize(
    params.ProjectID, params.ObjectID, params.UserIDInPath, params.FileID)
  ch <- isOkToAllow
  waitGroup.Done()
}

func extractParams(request events.APIGatewayProxyRequest) (*params) {
  // Path: /v1/projects/{project_id}/objects/{object_id}/users/{user_id}/files/{id_full}
  p := request.PathParameters
  projectID := p["project_id"]
  objectID := p["object_id"]
  userIDInPath := p["user_id"]
  fileID := p["file_id"]
  paramsStruct := params {
    ProjectID: projectID,
    ObjectID: objectID,
    UserIDInPath: userIDInPath,
    FileID : fileID,
  }
  return &paramsStruct
}

func userIDFromAuthHeader (authHeader string, userIDInPath string) (string, error){
  userIDFromAuthHeader, err := auth(authHeader)
  fmt.Printf("userIDFromAuthHeader: %s\n", userIDFromAuthHeader)
  fmt.Printf("userIDInPath: %s\n", userIDInPath)
  if userIDFromAuthHeader != userIDInPath {
    err = errInvalidHash
  }
  return userIDFromAuthHeader, err
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
  params := extractParams(request)
  if strings.HasSuffix(request.Path, "/upload") {
    return upload(request, params)
  }
  return download(request, params)
}

func buildErrorResponseForAuthHeader(err error, userIDInPath string, s3Key string, bucketName string) (events.APIGatewayProxyResponse) {
  var code int
  var body string
  switch err {
  case errNoUserHashs:
    code = 500; body = "Server setup does not finished. (Error code: 001)"
  case errInvalidHash:
    code = 403; body = "Invalid auth token given. (Error code: 002)"
  default:
    code = 500; body = "InternalServerError. (Error code: 005)"
  }
  log := fmt.Sprintf("userIDFromAuthHeader:%s", userIDFromAuthHeader)
  outputLog2CloudWatch(userIDInPath, s3Key, bucketName, log)
  return events.APIGatewayProxyResponse{
    Body      : fmt.Sprintf("{\"error\":\"%s\"}", body),
    StatusCode: code,
  }
}

func buildErrorResponseWithS3URL (userIDInPath string, s3Key string, bucketName string) (events.APIGatewayProxyResponse) {
  body := "Internal server error (Error code: 003)"
  outputLog2CloudWatch(userIDInPath, s3Key, bucketName, body)
  return events.APIGatewayProxyResponse{
    Body      : fmt.Sprintf("{\"error\":\"%s\"}", body),
    StatusCode: 500,
  }
}

func buildCannotLoggingToCloudWatchErrorResponse () (events.APIGatewayProxyResponse) {
  body := "Internal server error (Error code: 006)"
  return events.APIGatewayProxyResponse{
    Body      : fmt.Sprintf("{\"error\":\"%s\"}", body),
    StatusCode: 500,
  }
}

func download(request events.APIGatewayProxyRequest, params *params) (events.APIGatewayProxyResponse, error) {
  return workflow(request, params, "DL")
}

func upload(request events.APIGatewayProxyRequest, params *params) (events.APIGatewayProxyResponse, error) {
  return workflow(request, params, "UL")
}

func workflow(request events.APIGatewayProxyRequest, params *params, dlOrUl string) (events.APIGatewayProxyResponse, error) {
  var bucketName string
  var successCode int
  switch dlOrUl {
    case "DL": successCode = 302; bucketName = envS3DLBucketName
    case "UL": successCode = 200; bucketName = envS3ULBucketName
    default:
      pp.Print("Invalid dlOrUl given: %s", dlOrUl)
      return buildCannotLoggingToCloudWatchErrorResponse(), nil
  }
  // Check permission parallelly for time efficiency
  waitGroup := &sync.WaitGroup{}
  waitGroup.Add(1)
  checkPermissionCh := make(chan bool, 1)
  defer close(checkPermissionCh)  // https://qiita.com/convto/items/b2e95e549f35a1beb0b8
  go checkPermittedFileID(checkPermissionCh, waitGroup, params)

  s3Key := fmt.Sprintf("/%s/%s/%s", params.ProjectID, params.ObjectID, params.FileID)
  userIDFromAuthHeader, err := userIDFromAuthHeader(request.Headers["Authorization"], params.UserIDInPath)
  if userIDFromAuthHeader != params.UserIDInPath {
    err = errInvalidHash
  }
  if err != nil {
    return buildErrorResponseForAuthHeader(err, params.UserIDInPath, s3Key, bucketName), nil
  }
  fmt.Printf("s3Key: %s\n", s3Key)

  presignedURL, err := s3URLWithPreSign(dlOrUl, s3Key, bucketName, envAWSRegion)
  if err != nil {
    return buildErrorResponseWithS3URL(params.UserIDInPath, s3Key, bucketName), nil
  }
  waitGroup.Wait() // Wait for checking permissions in dynamodb
  isOkToAllow := <-checkPermissionCh
  if !isOkToAllow {
    body := "The requested file id is invalid for you. (Error code: 004)"
    outputLog2CloudWatch(params.UserIDInPath, s3Key, bucketName, body)
    return events.APIGatewayProxyResponse{
      Body      : fmt.Sprintf("{\"error\":\"%s\"}", body),
      StatusCode: 403,
    }, nil
  }

  outputLog2CloudWatch(params.UserIDInPath, s3Key, bucketName, "succeeded")
  respHeaders := map[string]string{}
  if dlOrUl == "DL" {
    respHeaders["Location"] = presignedURL
  }
  return events.APIGatewayProxyResponse{
    Body      : fmt.Sprintf("{\"url\":\"%s\"}", presignedURL),
    StatusCode: successCode,
    Headers   : respHeaders,
  }, nil
}

func main() {
  lambda.Start(handler)
}
