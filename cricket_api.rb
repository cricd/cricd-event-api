require 'date'
require 'sinatra/base'
require 'json'
require 'time'
require 'logger'

require_relative 'helpers'

$logger = Logger.new(STDOUT)

class CricketAPI < Sinatra::Base
  configure do
    set :bind, '0.0.0.0'
  end

  before do
    content_type :json
  end

  post '/event' do
    $logger.info("Received request from #{request.ip}")
    event = Helpers.parse_request(request)
    if event.nil?
      status 500
      body 'Internal server error'
      $logger.error("Failed to parse JSON #{e}")
      return
    end
    
    # Do the JSON validation
    valid = Helpers.validate_JSON(schema, event) 
    if !valid 
      $logger.info("Received request with invalid JSON ")
      status 400
      body 'Invalid JSON sent'
      return
    end
    
      # Do something useful with the JSON
      $logger.info("Request has valid JSON")
      pushed_to_ES = Helpers.push_to_ES(event)
      if !pushed_to_ES
        $logger.error("Failed to push event to EventStore - #{e}")
        status 500
        body 'Internal server error'
        return
      end
      $logger.info("Successfully pushed to EventStore with UUID - #{event_data[:event_id]}")
      
      # Get the next event
      next_event = Helpers.get_next_match_event(event)
      if next_event 
          status 201
          body response.body
          $logger.info("Successfully returned next ball event")
      else
          status 201
          body ""
          $logger.info("Failed to return next ball event")
      end
end
CricketAPI.run!
