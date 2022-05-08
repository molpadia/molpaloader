# molpaloader
The web server provides a feature of file uploading.

## Getting started
Generate the Tls certificate and private key via OpenSSL before launch the server connection.

```console
$ make certs SERVICE_NAME=localhost
```

Establish the server listen to the 4443 port on local environment.

```console
$ go run ./cmd/api --cert=./certs/localhost.cert.pem --key=./certs/localhost.key.pem
```
