require 'rspec'
require_relative '../helpers.rb'


describe Helpers do 

  before(:all) do
     @schema = JSON.parse(File.read('./event_schema.json'))
   end
  
     it "returns a returns true for an event with valid JSON schema" do
         expect(Helpers.validate_JSON(@schema, JSON.parse(File.read('./spec/good_event.json')) == true))
    end
    
    it "returns a returns false for an event with invalid JSON schema" do
         expect(Helpers.validate_JSON(@schema, JSON.parse(File.read('./spec/bad_event.json')) == true))
    end

    # it "returns nil when malformed JSON is provided" fo 
    #   expect(Helpers.parse_request())

    # end

  end
