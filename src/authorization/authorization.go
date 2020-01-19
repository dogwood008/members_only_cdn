package authorization

import (
  "os"
  "strconv"
  "regexp"

  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"

  "github.com/k0kubun/pp"
  "github.com/dogwood008/members_only_cdn/ddb"
)

var (
  envAWSRegion = os.Getenv("AWS_REGION")

  awsSession = session.New()
  awsConfig  = aws.NewConfig().WithRegion(envAWSRegion)
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

// Authorize and returns whether the user can access the file.
func Authorize(projectID string, objectID string, userID string, requestedFileID string) bool {
  requestedFileIDInt := convertAtoI(requestedFileID)
  permission, err := ddb.Fetch(projectID, objectID, userID)
  if err != nil {
    pp.Print(err)
    return false
  }
  allowedFileID := convertAtoI(permission.FileID)
  /*pp.Println(requestedFileIDInt)
  pp.Println(allowedFileID)
  pp.Println(permission)*/
  return requestedFileIDInt <= allowedFileID
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
