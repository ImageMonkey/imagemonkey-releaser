FROM golang:1.17.7-buster

RUN mkdir -p /home/go/bin
ENV GOPATH=/home/go
ENV GOBIN=/home/go/bin

RUN apt-get update \
	&& apt-get install -y --no-install-recommends \
    apt-transport-https ca-certificates curl gnupg2 software-properties-common

RUN curl -fsSL https://download.docker.com/linux/debian/gpg | apt-key add -
RUN apt-key fingerprint 0EBFCD88
RUN add-apt-repository \
   "deb [arch=amd64] https://download.docker.com/linux/debian \
   $(lsb_release -cs) \
   stable"

RUN apt-get update && apt-get install -y --no-install-recommends docker-ce docker-ce-cli containerd.io

RUN mkdir -p /tmp/src
COPY src/main.go /tmp/src/main.go
COPY src/go.mod /tmp/src/go.mod
COPY src/go.sum /tmp/src/go.sum

RUN cd /tmp/src && go build main.go && cp /tmp/src/main /home/go/bin/imagemonkeyreleaser
WORKDIR /home/go/bin/

ENTRYPOINT ["/home/go/bin/imagemonkeyreleaser"]
#ENTRYPOINT ["/tmp/entrypoint.sh"]
