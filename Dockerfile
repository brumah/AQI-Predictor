FROM golang:1.22

WORKDIR /app

COPY . .

RUN go build -o AQI_Predictor .

EXPOSE 8080

ENTRYPOINT [ "./AQI_Predictor" ]
