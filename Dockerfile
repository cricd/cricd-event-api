FROM golang:latest 
RUN mkdir /app 
ADD . /app/ 
WORKDIR /app 
RUN go get github.com/Sirupsen/logrus
RUN go get github.com/jetbasrawi/go.geteventstore
RUN go get github.com/xeipuuv/gojsonschema
RUN go get github.com/gorilla/mux

RUN go build -o event_api.go . 
CMD ["/app/event_api"]

EXPOSE 4567
