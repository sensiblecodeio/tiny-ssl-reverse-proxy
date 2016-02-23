fakecert: .FORCE
	openssl req \
		-x509 \
		-nodes \
		-sha256 \
		-newkey rsa:2048 \
		-keyout key.pem \
		-out crt.pem \
		-subj '/L=Earth/O=Fake Certificate/CN=localhost/' \
		-days 365

.FORCE:
