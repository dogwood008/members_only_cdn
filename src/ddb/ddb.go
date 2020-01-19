package ddb

import (
  "fmt"
  "os"
  "time"
  "errors"

  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"

  "github.com/guregu/dynamo"
  // for debug: "github.com/k0kubun/pp"
)

// Permission table of users
type Permission struct {
  UserID     string `dynamo:"user_id"`
  ProjectID  string `dynamo:"project_id"`
  ObjectID   string `dynamo:"object_id"`
  FileID     string `dynamo:"file_id"`
  UpdatedAt  time.Time `dynamo:"updated_at"`
  ProjectIDAndObjectID  string `dynamo:"project_id_and_object_id"`
}

var (
  errNoTableNameGiven = errors.New("No table name given")
  envDynamoDBTableName = os.Getenv("DYNAMO_DB_TABLE_NAME")

  envAWSRegion = os.Getenv("AWS_REGION")

  awsSession = session.New()
  awsConfig  = aws.NewConfig().WithRegion(envAWSRegion)

  ddb = dynamo.New(awsSession, awsConfig)
  table = ddb.Table(envDynamoDBTableName)
)

const (
  partitionKeyName = "user_id"
  sortKeyName = "project_id_and_object_id"
)

// Fetch from DynamoDB by projectID, objectID, userID.
func Fetch(projectID string, objectID string, userID string) (*Permission, error){
  if len(envDynamoDBTableName) == 0 {
    return nil, errNoTableNameGiven
  }

  partitionKey := userID
  sortKey := fmt.Sprintf("%s_%s", projectID, objectID)

  var result Permission
  err := table.Get(partitionKeyName, partitionKey).Range(sortKeyName, dynamo.Equal, sortKey).One(&result)
  return &result, err
}

/* for debug
func main() {
  resp, _ := Fetch("a", "b", "001")
  pp.Print(resp)
}
*/
