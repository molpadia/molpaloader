# molpastream
The web server provides a feature of video stream uploading.

## Getting started
Generate the Tls certificate and private key via OpenSSL before launch the server connection.

```console
$ make certs SERVICE_NAME=localhost
```

Establish the server listen to the 4443 port on local environment.

```console
$ docker build -t molpastream .
$ docker run -it -p 4443:4443 --env-file .env -v /Users/mongchelee/Public/development/projects/molpastream/certs:/var/lib/certs molpastream
```
