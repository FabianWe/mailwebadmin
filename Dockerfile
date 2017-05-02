FROM golang:1.8
MAINTAINER Fabian Wenzelmann <fabianwen@posteo.eu>

COPY docker_entrypoint.sh /
RUN chmod +x /docker_entrypoint.sh

COPY . $GOPATH/src/github.com/FabianWe/mailwebadmin

WORKDIR $GOPATH/src/github.com/FabianWe/mailwebadmin

RUN go get -v -d ...


RUN cd cmd/mailwebadmin && go install -v

CMD /docker_entrypoint.sh
