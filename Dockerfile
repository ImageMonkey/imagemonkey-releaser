FROM golang:1.12.5-stretch

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

COPY src/main.go /tmp/main.go
COPY src/go.mod /tmp/go.mod

RUN cd /tmp/ && go build main.go && cp /tmp/main /home/go/bin/imagemonkeyreleaser
WORKDIR /home/go/bin/

ENTRYPOINT ["/home/go/bin/imagemonkeyreleaser"]
#ENTRYPOINT ["/tmp/entrypoint.sh"]
