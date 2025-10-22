FROM golang:1.24

WORKDIR /app
COPY . .

# Download dependencies
RUN go mod tidy

# Build binary
RUN go build -o /go/bin/ethexporter .

ENV GETH=https://mainnet.infura.io
ENV PORT=9015

#COPY addresses.txt /app

EXPOSE 9015

ENTRYPOINT ["/go/bin/ethexporter"]
