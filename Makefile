certs:
	openssl genrsa -out ./certs/${SERVICE_NAME}.key.pem 2048
	openssl req \
		-new -x509 \
		-days 3650 \
		-key ./certs/${SERVICE_NAME}.key.pem \
		-out ./certs/${SERVICE_NAME}.cert.pem \
		-subj /CN=localhost \
		-addext "subjectAltName = DNS:localhost"

.PHONY: fmt install grpc certs
