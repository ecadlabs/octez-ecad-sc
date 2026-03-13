FROM golang:1.24-alpine
ADD . /octez-ecad-sc
WORKDIR /octez-ecad-sc

RUN go build -o octez-ecad-sc

ENTRYPOINT ["/octez-ecad-sc/octez-ecad-sc"]
CMD [ "-c", "/octez-ecad-sc/config.yaml" ]
