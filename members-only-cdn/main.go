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
	// ErrNoIP No IP found in response
	ErrNoIP = errors.New("No IP in HTTP response")

	// ErrNon200Response non 200 status code in response
	ErrNon200Response = errors.New("Non 200 Response found")

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

type Params struct {
  ProjectId    string
  ObjectId     string
  UserIdInPath string
  FileId       string
}


// https://stackoverflow.com/questions/40326540/how-to-assign-default-value-if-env-var-is-empty
func getEnv(key, fallback string) string {
    if value, ok := os.LookupEnv(key); ok {
        return value
    }
    return fallback
}

func userId(hash string, jsonString string) (string, error) {
  if jsonString == "" {
    return "", errNoUserHashs
  }
  var extractedUserId string
  var intf interface{}
  bytes := []byte(jsonString)
  json.Unmarshal(bytes, &intf)
  hmm := intf.(map[string]interface{})
  hmmm :=hmm["Maps"].(map[string]interface{})

  uncastUid := hmmm[hash]
  if uncastUid == nil {
    return "", errInvalidHash
  }
  extractedUserId = uncastUid.(string)
  return extractedUserId, nil
}

func auth(authHeader string) (string, error) {
  var hexToken, authRawToken string
  authRawToken = strings.Replace(authHeader, "Bearer ", "", 1)
  bytes := sha256.Sum256([]byte(authRawToken))
  hexToken = hex.EncodeToString(bytes[:])
  uid, err := userId(hexToken, envMapJSONString)
  return uid, err
}

func s3GetUrlWithPreSign (keyName string, bucketName string, region string) (string, error) {
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

func s3UrlWithPreSign (dlOrUl string, keyName string, bucketName string, region string) (string, error) {
  switch dlOrUl {
    case "DL": return s3GetUrlWithPreSign(keyName, bucketName, region)
    case "UL": return s3PutUrlWithPreSign(keyName, bucketName, region)
  }
  return "", errInvalidDlOrUl
}

// https://docs.aws.amazon.com/sdk-for-go/api/service/s3/#PutObjectInput
// https://docs.aws.amazon.com/sdk-for-go/api/service/s3/#S3.PutObjectRequest
// https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#CannedACL
// https://www.whatsajunting.com/posts/s3-presigned/
func s3PutUrlWithPreSign (keyName string, bucketName string, region string) (string, error) {
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

func outputLog2CloudWatch (userId string, s3Key string, bucketName string, err string) {
  log := fmt.Sprintf(",%s,\"s3://%s%s\",\"%s\"", userId, bucketName, s3Key, err)
  cloudWatchLogs.OutputLog2CloudWatch(&log)
}

func checkPermittedFileId (ch chan<- bool, waitGroup *sync.WaitGroup, params *Params) {
  isOkToAllow := authorization.Authorize(
    params.ProjectId, params.ObjectId, params.UserIdInPath, params.FileId)
  ch <- isOkToAllow
  waitGroup.Done()
}

func params(request events.APIGatewayProxyRequest) (*Params) {
  // Path: /projects/{project_id}/objects/{object_id}/users/{user_id}/files/{id_full}
  params := request.PathParameters
  projectId := params["project_id"]
  objectId := params["object_id"]
  userIdInPath := params["user_id"]
  fileId := params["file_id"]
  paramsStruct := Params {
    ProjectId: projectId,
    ObjectId: objectId,
    UserIdInPath: userIdInPath,
    FileId : fileId,
  }
  return &paramsStruct
}

func userIdFromAuthHeader (authHeader string, userIdInPath string) (string, error){
  userIdFromAuthHeader, err := auth(authHeader)
  fmt.Printf("userIdFromAuthHeader: %s\n", userIdFromAuthHeader)
  fmt.Printf("userIdInPath: %s\n", userIdInPath)
  if userIdFromAuthHeader != userIdInPath {
    err = errInvalidHash
  }
  return userIdFromAuthHeader, err
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
  params := params(request)
  if strings.HasSuffix(request.Path, "/upload") {
    return upload(request, params)
  } else {
    return download(request, params)
  }
}

func buildErrorResponseForAuthHeader(err error, userIdInPath string, s3Key string, bucketName string) (events.APIGatewayProxyResponse) {
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
  log := fmt.Sprintf("userIdFromAuthHeader:%s", userIdFromAuthHeader)
  outputLog2CloudWatch(userIdInPath, s3Key, bucketName, log)
  return events.APIGatewayProxyResponse{
    Body      : fmt.Sprintf("{\"error\":\"%s\"}", body),
    StatusCode: code,
  }
}

func buildErrorResponseWithS3Url (userIdInPath string, s3Key string, bucketName string) (events.APIGatewayProxyResponse) {
  body := "Internal server error (Error code: 003)"
  outputLog2CloudWatch(userIdInPath, s3Key, bucketName, body)
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

func download(request events.APIGatewayProxyRequest, params *Params) (events.APIGatewayProxyResponse, error) {
  return workflow(request, params, "DL")
}

func upload(request events.APIGatewayProxyRequest, params *Params) (events.APIGatewayProxyResponse, error) {
  return workflow(request, params, "UL")
}

func workflow(request events.APIGatewayProxyRequest, params *Params, dlOrUl string) (events.APIGatewayProxyResponse, error) {
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
  go checkPermittedFileId(checkPermissionCh, waitGroup, params)

  s3Key := fmt.Sprintf("/%s/%s/%s", params.ProjectId, params.ObjectId, params.FileId)
  userIdFromAuthHeader, err := userIdFromAuthHeader(request.Headers["Authorization"], params.UserIdInPath)
  if userIdFromAuthHeader != params.UserIdInPath {
    err = errInvalidHash
  }
  if err != nil {
    return buildErrorResponseForAuthHeader(err, params.UserIdInPath, s3Key, bucketName), nil
  }
  fmt.Printf("s3Key: %s\n", s3Key)

  presignedUrl, err := s3UrlWithPreSign(dlOrUl, s3Key, bucketName, envAWSRegion)
  if err != nil {
    return buildErrorResponseWithS3Url(params.UserIdInPath, s3Key, bucketName), nil
  }
  waitGroup.Wait() // Wait for checking permissions in dynamodb
  isOkToAllow := <-checkPermissionCh
  if !isOkToAllow {
    body := "The requested file id is invalid for you. (Error code: 004)"
    outputLog2CloudWatch(params.UserIdInPath, s3Key, bucketName, body)
    return events.APIGatewayProxyResponse{
      Body      : fmt.Sprintf("{\"error\":\"%s\"}", body),
      StatusCode: 403,
    }, nil
  }

  outputLog2CloudWatch(params.UserIdInPath, s3Key, bucketName, "succeeded")
  respHeaders := map[string]string{}
  if dlOrUl == "DL" {
    respHeaders["Location"] = presignedUrl
  }
	return events.APIGatewayProxyResponse{
    Body      : fmt.Sprintf("{\"url\":\"%s\"}", presignedUrl),
		StatusCode: successCode,
    Headers   : respHeaders,
	}, nil
}

func main() {
	lambda.Start(handler)
}
