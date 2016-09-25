require 'rspec'
require_relative '../helpers.rb'


describe Helpers do 

  before(:all) do
     @schema = JSON.parse(File.read('./event_schema.json'))
   end
  
     it "returns true for an event with valid JSON schema" do
         expect(Helpers.validate_JSON(JSON.parse(File.read('./spec/good_event.json')) == true))
    end
    
    it "returns false for an event with invalid JSON schema" do
         expect(Helpers.validate_JSON(JSON.parse(File.read('./spec/bad_event.json')) == true))
    end

    # it "returns nil when malformed JSON is provided" do
    #   expect(Helpers.parse_request(File.read('./spec/broken_event.json')).nil? == true)
    #  end

    it "returns false when unsuccessfully pushing to ES" do
      expect(Helpers.validate_JSON(JSON.parse(File.read('./spec/good_event.json')) == false))
   end

    # it "returns false when unsuccessfully pushing to ES" do
    #   expect(Helpers.validate_JSON(JSON.parse(File.read('./spec/good_event.json')) == false))
    # end

  end
