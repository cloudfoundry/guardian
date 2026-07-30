In order to regenerate the certificate you need to execute the following commands:
* `openssl req -new -key cert.key -out newcsr.csr -config openssl-generate-cert.cnf`
* `openssl x509 -req -days 3650 -in newcsr.csr -signkey cert.key -out cert.cert -extensions v3_req -extfile openssl-generate-cert.cnf` 
