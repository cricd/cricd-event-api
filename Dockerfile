FROM golang:latest 
RUN mkdir /app 
ADD . /app/ 
WORKDIR /app 
RUN go get github.com/Sirupsen/logrus
RUN go get github.com/jetbasrawi/go.geteventstore
RUN go get github.com/xeipuuv/gojsonschema
RUN go get github.com/gorilla/mux

EXPOSE 4567
RUN go build -o event_api.go . 
CMD ["/app/event_api"]

