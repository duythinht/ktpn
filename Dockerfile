FROM golang:1.25.3-trixie AS builder
RUN apt-get update && apt-get install -y -qq libleptonica-dev libtesseract-dev
WORKDIR /opt/src/ktpn/
COPY go.mod go.sum /opt/src/ktpn/
RUN go mod download
COPY ./ /opt/src/ktpn/
RUN go build -o /opt/bin/ktpn ./cmd/ktpn

FROM debian:trixie
RUN apt-get update && apt-get install -y -qq libleptonica-dev libtesseract-dev tesseract-ocr-eng
COPY --from=builder /opt/bin/ktpn /usr/local/bin/ktpn
CMD [ "ktpn" ]
