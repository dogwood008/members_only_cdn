# members_only_cdn

## What is this?

This is a Golang sample SAM project that made from template.

## What can this do?

API Gateway を通して Lambdaへアクセスすると、S3の指定のファイルへ10分間だけアクセスできるURLを発行します。

302で応答するので、curlの `--location` オプションを使用すれば、自動的にS3からcurlがダウンロードしてくれます。

リクエストを受けた際、リクエストヘッダ内のBearerトークンを検証し、認可情報を Dynamo DB から取得して、本当に返答して良いかを確認した上で応答します。

---

Request to a lambda through API Gateway, the lambda responsed 302 with a S3 pre-signed signature URL which is valid only in 10 minutes.

You can get files on S3 which only authorized users can see like this:

```bash
curl -H 'Authorization: Bearer abc' http://localhost:3000/projects/a/objects/b/users/001/files/001.csv --location
```

**NOTE:** There is a [vulnerability bug](https://curl.haxx.se/docs/CVE-2018-1000007.html) ([explanation](https://stackoverflow.com/a/50005430)) in `curl < 7.58.0`.
If you use the curl which version is less than 7.58.0, you will get `400` from S3 server because of multiple auth like "`Only one auth mechanism allowed`".


## Directories

```bash
.
├── LICENSE
├── README.md
├── members-only-cdn
│   ├── Makefile
│   ├── authorization
│   │   └── authorization.go
│   ├── cwlogs
│   │   └── cwlogs.go
│   ├── ddb
│   │   └── ddb.go
│   ├── go.mod
│   ├── go.sum
│   ├── main.go
│   ├── main_test.go  <- Not yet implemented
│   └── swagger.yaml  <- API specification
└── template.yaml     <- write Env here
```

## Requirements

* AWS CLI already configured with Administrator permission
* [Docker installed](https://www.docker.com/community-edition)
* [Golang](https://golang.org)
  * I used `go1.13.5`

## Setup process

### Environment variables

There are defined in `template.yaml`.

- `USER_TOKEN_MAP_JSON`
  - `{"Maps":{"sha256hashed_password": "user_id", ...}}`
- `BUCKET_NAME`
  - S3 bucket name from which users will download files
- `AWS_REGION`
  - Refer [instructions](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html#concepts-available-regions)
- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `DYNAMO_DB_TABLE_NAME`
  - The name of table in Dynamo DB in which permissions are saved
- `CLOUD_WATCH_LOG_GROUP_NAME`
  - Log group name of CloudWatch Logs to save log
- `CLOUD_WATCH_ENABLE_SETUP`
  - Whether create log stream and log group or not when which doesn't exist

### Installing dependencies

<details><summary>These instructions may not be unnecessary</summary>
<p>
In this example we use the built-in `go get` and the only dependency we need is AWS Lambda Go SDK:

```shell
go get -u github.com/aws/aws-lambda-go/...
```
</p>
</details>

**NOTE:** As you change your application code as well as dependencies during development, you might want to research how to handle dependencies in Golang at scale.

### Building

Golang is a statically compiled language, meaning that in order to run it you have to build the executable target.

You can issue the following command in a shell to build it:

```shell
cd members-only-cdn
make build
```

**NOTE**: If you're not building the function on a Linux machine, you will need to specify the `GOOS` and `GOARCH` environment variables, this allows Golang to build your function for another system architecture and ensure compatibility.

### Local development

**Invoking function locally through local API Gateway**

```bash
sam local start-api
```

If the previous command ran successfully you should now be able to hit the following local endpoint to invoke your function
```
http://localhost:3000/projects/{project_id}/objects/{object_id}/users/{user_id}/files/{file_id}

e.g.)
http://localhost:3000/projects/foo/objects/bar/users/001/files/12345
```
**NOTE**: You have to add bearer token to header as this:
```shell
curl -H 'Authorization: Bearer abc' http://localhost:3000/projects/foo/objects/bar/users/001/files/12345.csv
```
Then, you will get a URL which is S3 pre-signed URL like this:

```
https://dogwood008-members-only-cdn.s3.us-west-2.amazonaws.com/a/b/001asdb.csv?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKIAFOOBARBAZ%2F20200102%2Fus-west-2%2Fs3%2Faws4_request&X-Amz-Date=20200102T171223Z&X-Amz-Expires=600&X-Amz-SignedHeaders=host&X-Amz-Signature=aee220359eec8a34262e641dff1990500123456789abcdef0123456789abcdef
```

**SAM CLI** is used to emulate both Lambda and API Gateway locally and uses our `template.yaml` to understand how to bootstrap this environment (runtime, where the source code is, etc.). API specification is written by OpenAPI 3.0. It is in `./members-only-cdn/swagger.yaml`.


## Packaging and deployment

AWS Lambda Python runtime requires a flat folder with all dependencies including the application. SAM will use `CodeUri` property to know where to look up for both application and dependencies:

```yaml
...
    MembersOnlyCdnFunction:
        Type: AWS::Serverless::Function
        Properties:
            CodeUri: members-only-cdn/ # <- links to directory: `./members-only-cdn/`
            Handler: members-only-cdn  # <- links to `go build -o HERE main.go`
            ...
```

To deploy your application for the first time, run the following in your shell:

```bash
sam deploy --guided
```

The command will package and deploy your application to AWS, with a series of prompts:

* **Stack Name**: The name of the stack to deploy to CloudFormation. This should be unique to your account and region, and a good starting point would be something matching your project name.
* **AWS Region**: The AWS region you want to deploy your app to.
* **Confirm changes before deploy**: If set to yes, any change sets will be shown to you before execution for manual review. If set to no, the AWS SAM CLI will automatically deploy application changes.
* **Allow SAM CLI IAM role creation**: Many AWS SAM templates, including this example, create AWS IAM roles required for the AWS Lambda function(s) included to access AWS services. By default, these are scoped down to minimum required permissions. To deploy an AWS CloudFormation stack which creates or modified IAM roles, the `CAPABILITY_IAM` value for `capabilities` must be provided. If permission isn't provided through this prompt, to deploy this example you must explicitly pass `--capabilities CAPABILITY_IAM` to the `sam deploy` command.
* **Save arguments to samconfig.toml**: If set to yes, your choices will be saved to a configuration file inside the project, so that in the future you can just re-run `sam deploy` without parameters to deploy changes to your application.

You can find your API Gateway Endpoint URL in the output values displayed after deployment.

### Testing

<details><summary>Not yet implemented.</summary>
<p>
We use `testing` package that is built-in in Golang and you can simply run the following command to run our tests:

```shell
go test -v ./hello-world/
```
</p>
</details>

# Appendix

### Golang installation

Please ensure Go 1.x (where 'x' is the latest version) is installed as per the instructions on the official golang website: https://golang.org/doc/install

A quickstart way would be to use Homebrew, chocolatey or your linux package manager.

#### Homebrew (Mac)

Issue the following command from the terminal:

```shell
brew install golang
```

If it's already installed, run the following command to ensure it's the latest version:

```shell
brew update
brew upgrade golang
```

#### Chocolatey (Windows)

Issue the following command from the powershell:

```shell
choco install golang
```

If it's already installed, run the following command to ensure it's the latest version:

```shell
choco upgrade golang
```

## Bringing to the next level

Here are a few ideas that you can use to get more acquainted as to how this overall process works:

* Create an additional API resource (e.g. /hello/{proxy+}) and return the name requested through this new path
* Update unit test to capture that
* Package & Deploy

Next, you can use the following resources to know more about beyond hello world samples and how others structure their Serverless applications:

* [AWS Serverless Application Repository](https://aws.amazon.com/serverless/serverlessrepo/)
