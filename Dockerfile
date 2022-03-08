FROM scratch
ADD https://curl.se/ca/cacert.pem /etc/ssl/certs/ca-certificates.crt
ADD ./workspace/satisfy /satisfy
CMD ["/satisfy"]