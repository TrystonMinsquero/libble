FROM golang:1.25-alpine
# FROM golang:1.16-alpine AS build

WORKDIR /libble
COPY ./server ./server
COPY ./shared ./shared
COPY ./go.mod .
COPY ./go.sum .

RUN go mod download
RUN go build -o ./main ./server

# FROM alpine:latest

# WORKDIR /libble
# COPY --from=build /libble/main .

EXPOSE 8080

CMD ["./main"]
