# cricd-event-api

This HTTP API provides a POST endpoint to submit a cricd event to EventStore. The expected payload is a JSON document that complies to the agreed upon schema. 


## Running the service in Docker

This API is dependent on a connection to an EventStore to store the cricd events . The ip, port and Eventstore stream name are all configured using the following environment variables: EVENTSTORE_IP, EVENTSTORE_PORT, EVENTSTORE_STREAM_NAME

You can specify these environment variables when running the docker container. For example docker run -d -p 4567:4567 -e EVENTSTORE_IP=172.18.0.2 ryankscott/cricd-event-api

If your EventStore instance is running in a Docker container as well then network connectivity will need to be established between these instances. This is explained in the Docker networking documentation but the steps at a high level are: 1. Create a user defined network using a command like docker network create --driver bridge cricd-network 2. Start your EventStore container using the --network parameter docker run --net=cricd-network 3. Find the IP address of the EventStore
container using the command docker network inspect cricd-network 4. Start this Docker container using the --net=cricd-network parameter and using the EVENTSTORE_IP variable set to the IP address you just found

Alternatively, you can clone the code repository for this service and use Docker-Compose to spin up a environment containing both EventStore and this service which removes the need to perform these steps

##Accessing the service

This service exposes a single endpoint at port 4567 by default which responds to POST requests. The service expects a JSON payload of the event

For example: http://localhost:4567/event

