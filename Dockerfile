FROM golang:latest

RUN apt-get update -qq

RUN apt-get install -y -qq libtesseract-dev libleptonica-dev
ENV TESSDATA_PREFIX=/usr/share/tesseract-ocr/5/tessdata/
RUN apt-get install -y -qq tesseract-ocr-deu tesseract-ocr-eng

WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go get github.com/otiai10/gosseract/v2
RUN go mod download
COPY *.go ./
RUN go build -o /ocr

CMD [ "/ocr" ]