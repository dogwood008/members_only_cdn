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

  EnvDynamoDBTableName = os.Getenv("DYNAMO_DB_TABLE_NAME")

  EnvAWSRegion = os.Getenv("AWS_REGION")

  awsSession = session.New()
  awsConfig  = aws.NewConfig().WithRegion(EnvAWSRegion)

  ddb = dynamo.New(awsSession, awsConfig)
  table = ddb.Table(EnvDynamoDBTableName)
)

const (
  partitionKeyName = "user_id"
  sortKeyName = "project_id_and_object_id"
)


func fetch(projectId string, objectId string, userId string) (*Permission, error){
  if len(EnvDynamoDBTableName) == 0 {
    return nil, ErrNoTableNameGiven
  }

  partitionKey := userId
  sortKey := fmt.Sprintf("%s_%s", projectId, objectId)

  var result Permission
  err := table.Get(partitionKeyName, partitionKey).Range(sortKeyName, dynamo.Equal, sortKey).One(&result)
  return &result, err
}

/* for debug
func main() {
  resp, _ := fetch("a", "b", "001")
  pp.Print(resp)
}
*/
