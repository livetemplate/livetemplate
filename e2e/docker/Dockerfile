FROM golang:1.26-alpine AS builder
WORKDIR /src

# Copy the library source (needed for replace directive)
COPY . /src/livetemplate/

# Build the test app
WORKDIR /src/livetemplate/e2e/docker/app
# go.sum is not committed for the e2e modules, so resolve it in the image.
RUN GOWORK=off go mod tidy && GOWORK=off CGO_ENABLED=0 go build -o /chat-server .

FROM alpine:3.21
COPY --from=builder /chat-server /chat-server
RUN mkdir -p /app && echo "" > /app/chat.html
WORKDIR /app
EXPOSE 8080
CMD ["/chat-server"]
