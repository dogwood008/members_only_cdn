package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
  "strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var (
	// DefaultHTTPGetAddress Default Address
	DefaultHTTPGetAddress = "https://checkip.amazonaws.com"

	// ErrNoIP No IP found in response
	ErrNoIP = errors.New("No IP in HTTP response")

	// ErrNon200Response non 200 status code in response
	ErrNon200Response = errors.New("Non 200 Response found")
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	resp, err := http.Get(DefaultHTTPGetAddress)

  fmt.Print(request.PathParameters)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	if resp.StatusCode != 200 {
		return events.APIGatewayProxyResponse{}, ErrNon200Response
	}

	ip, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	if len(ip) == 0 {
		return events.APIGatewayProxyResponse{}, ErrNoIP
	}

  // Path: /{contest_name}/{object_name}/{user_id}/{id_full}
  params := request.PathParameters
  contest_name := params["contest_name"]
  object_name := params["object_name"]
  user_id := params["user_id"]
  id_full := strings.Replace(params["id_full"], ".csv", "", 1)
  gen_path := fmt.Sprintf("/%s/%s/%s/%s.csv", contest_name, object_name, user_id, id_full)

  resp_body := fmt.Sprintf("Hello, %v", string(ip)) + gen_path
	return events.APIGatewayProxyResponse{
    Body      : resp_body,
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
