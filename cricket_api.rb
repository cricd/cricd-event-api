require 'date'
require 'sinatra/base'
require 'json'
require 'http_eventstore'
require 'time'
require 'logger'
require 'json-schema'
require 'securerandom'
require 'httparty'

$logger = Logger.new(STDOUT)

# Pull the settings from ENV variables
$settings = {
  :event_store_ip => ENV["EVENTSTORE_IP"].nil? ? "localhost" : ENV["EVENTSTORE_IP"],
  :event_store_port => ENV["EVENTSTORE_PORT"].nil? ? "2113" : ENV["EVENTSTORE_PORT"],
  :stream_name => ENV["EVENTSTORE_STREAM_NAME"].nil? ? "cricket_events_v1" : ENV["EVENTSTORE_STREAM_NAME"],
  :next_ball_ip => ENV["NEXT_BALL_IP"].nil? ? "localhost" : ENV["NEXT_BALL_IP"],
  :next_ball_port => ENV["NEXT_BALL_PORT"].nil? ? "3004" : ENV["NEXT_BALL_PORT"]
}


# Set up ES
$client = HttpEventstore::Connection.new do |config|
  config.endpoint = $settings[:event_store_ip]
  config.port = $settings[:event_store_port]
  config.page_size = '50'
end
  $stream_name = $settings[:stream_name]


class App < Sinatra::Base
  configure do
    set :bind, '0.0.0.0'
  end

  before do
    content_type :json
  end

# Get JSON schema
  begin
    schema = JSON.parse(File.read('event_schema.json'))
  rescue IOError => e
    $logger.fatal("Unable to open or parse JSON schema #{e}")
    exit
  end

  post '/event' do
    $logger.info("Received request from #{request.ip}")
    begin
      event = JSON.parse(request.body.read)
    rescue JSON::ParserError => e
      status 500
      body 'Internal server error'
      $logger.error("Failed to parse JSON #{e}")
    end 

    # Do the JSON validation
    valid = JSON::Validator.validate(schema, event)
    if !valid 
      $logger.info("Received request with invalid JSON ")
      status 400
      body 'Invalid JSON sent'
      return
    else
      # Do something useful with the JSON
      begin
      $logger.info("Request has valid JSON")
      event_data = { event_type: "cricket_event",
                      data: event,
                      event_id: SecureRandom.uuid
                      }
      $client.append_to_stream($stream_name, event_data)
      rescue StandardError => e
        status 500
        body 'Internal server error'
        $logger.error("Failed to push event to EventStore - #{e}")
        return
      end
      $logger.info("Successfully pushed to EventStore with UUID - #{event_data[:event_id]}")
      
      # Get the next event
      match_id = event["match"].to_s
      uri = "http://" + + $settings[:next_ball_ip] + ":" +  $settings[:next_ball_port]
      response = HTTParty.get("#{uri}",
                             :query => {'match' => match_id},
                             :headers => { 'Content-Type' => 'application/json'},
                             :timeout => 1)
      case response.code
        when 200
          status 201
          body response.body
          $logger.info("Successfully returned next ball event")
          return
        else
          status 201
          body ""
          $logger.info("Failed to return next ball event")
          return nil
        end
      end

  end
end
App.run!
