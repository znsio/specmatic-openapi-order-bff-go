   # Use the official Golang image as the base image
   FROM golang:1.22.4-alpine

   # Set the Current Working Directory inside the container
   WORKDIR /app

   # Copy go mod and sum files
   COPY go.mod go.sum ./

   # Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
   RUN go mod download && \
    apk add --no-cache curl

   # Copy the source from the current directory to the Working Directory inside the container
   COPY . .

   # Build the Go app
   RUN go build -o main ./cmd/main.go

   # Expose port 8080 to the outside world
   EXPOSE 8080

   # Command to run the executable
   CMD ["./main"]