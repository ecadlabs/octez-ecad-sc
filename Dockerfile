FROM golang:1.22-alpine3.20
ADD . /octez-ecad-sc
WORKDIR /octez-ecad-sc

RUN go build -o octez-ecad-sc

ENTRYPOINT ["/octez-ecad-sc/octez-ecad-sc"]
CMD [ "-c", "/octez-ecad-sc/config.yaml" ]
