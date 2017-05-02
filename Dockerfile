FROM golang:1.8
MAINTAINER Fabian Wenzelmann <fabianwen@posteo.eu>


COPY . $GOPATH/src/github.com/FabianWe/mailwebadmin

WORKDIR $GOPATH/src/github.com/FabianWe/mailwebadmin

RUN go get -v -d ...

RUN go install -v github.com/FabianWe/mailwebadmin/...

RUN mkdir -p /config && mkdir -p /backup && mkdir -p /var/vmail

COPY docker_entrypoint.sh /
RUN chmod +x /docker_entrypoint.sh

ENTRYPOINT ["/docker_entrypoint.sh"]
CMD ["mailwebadmin", "-config", "/config"]
