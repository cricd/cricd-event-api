require 'json-schema'
require 'json'
require 'logger'
require 'securerandom'
require 'http_eventstore'

module Helpers
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
    $logger = Logger.new(STDOUT)

    begin
        $schema = JSON.parse(File.read('event_schema.json'))
    rescue IOError => e
        $logger.fatal("Unable to open or parse JSON schema #{e}")
        exit 
    end

    def self.parse_request(request)
        begin
            event = JSON.parse(request.body.read)
        rescue JSON::ParserError => e
            return nil
        end 
        return event
    end

    def self.validate_JSON(event)
        valid = JSON::Validator.validate($schema, event)
        if valid 
            return true
        else
            return false
        end
    end

    def self.push_to_ES(event)
        begin
            event_data = { event_type: "cricket_event",
                        data: event,
                        event_id: SecureRandom.uuid
                        }
            $client.append_to_stream($stream_name, event_data)
        rescue StandardError => e
            return false
        end
        return true
    end

    def self.get_next_match_event(event)
        match_id = event["match"].to_s
        uri = "http://" + + $settings[:next_ball_ip] + ":" +  $settings[:next_ball_port]
        response = HTTParty.get("#{uri}",
                             :query => {'match' => match_id},
                             :headers => { 'Content-Type' => 'application/json'},
                             :timeout => 1)
        case response.code
            when 200
                return response.body
            else
                return nil
            end
        end
end
