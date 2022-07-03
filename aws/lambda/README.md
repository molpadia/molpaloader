## AWS Lambda function

Build AWS lambda functions to trigger event processing.

### Getting started
```console
# Note the output executable name is in correspondence with lambda function handler.
$ GOOS=linux GOARCH=amd64 go build -o main main.go
$ zip lambda.zip main
```
