package authorization

import (
  "os"
  "time"
  "errors"
  "strconv"
  "regexp"

  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"

  "github.com/k0kubun/pp"
  "github.com/dogwood008/members_only_cdn/ddb"
)

type Permission struct {
  UserId     string `dynamo:"user_id"`
  ProjectId  string `dynamo:"project_id"`
  ObjectId   string `dynamo:"object_id"`
  FileId     string `dynamo:"file_id"`
  UpdatedAt  time.Time `dynamo:"updated_at"`
  ProjectIdAndObjectId  string `dynamo:"project_id_and_object_id"`
}

var (
	// ErrNoIP No IP found in response
	ErrNoTableNameGiven = errors.New("No table name given.")

  EnvAWSRegion = os.Getenv("AWS_REGION")

  awsSession = session.New()
  awsConfig  = aws.NewConfig().WithRegion(EnvAWSRegion)
)

const (
  partitionKeyName = "user_id"
  sortKeyName = "project_id_and_object_id"
)

// https://teratail.com/questions/99069
func convertAtoI(str string) int {
  var regex = regexp.MustCompile(`\d+`)
  value, _ := strconv.Atoi(regex.FindString(str))
  return value
}

func Authorize(projectId string, objectId string, userId string, requestedFileId string) bool {
  requestedFileIdInt := convertAtoI(requestedFileId)
  permission, err := ddb.Fetch(projectId, objectId, userId)
  if err != nil {
    pp.Print(err)
    return false
  }
  allowedFileId := convertAtoI(permission.FileId)
  /*pp.Println(requestedFileIdInt)
  pp.Println(allowedFileId)
  pp.Println(permission)*/
  return requestedFileIdInt <= allowedFileId
}

func init () {
  if os.Getenv("ENABLE_COLOR_PP") == "false" {
    // https://github.com/k0kubun/pp/issues/26#issuecomment-544108705
    pp.ColoringEnabled = false
  }
}
/* for debug
func main() {
  resp := Authorize("a", "b", "001", "000.csv")
  pp.Print(resp)
}
//*/
