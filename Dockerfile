
FROM ruby:2.3
MAINTAINER Ryan Scott <ryankennethscott@gmail.com>

# throw errors if Gemfile has been modified since Gemfile.lock
RUN bundle config --global frozen 1

RUN mkdir /app
WORKDIR /app
COPY Gemfile /app/
COPY Gemfile.lock /app/
RUN bundle install

COPY . /app

EXPOSE 4567
CMD ["ruby", "cricket_api.rb"]



