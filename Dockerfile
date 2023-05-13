FROM golang:latest

COPY . /bamboo-stratus

WORKDIR /bamboo-stratus

RUN go env -w GOPROXY=https://goproxy.cn,direct && go get -d -v ./... && go build

EXPOSE 8070
EXPOSE 3735

