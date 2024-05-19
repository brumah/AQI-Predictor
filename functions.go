package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
)

type Data struct {
	Main Main `json:"main"`
	Wind Wind `json:"wind"`
}

type Main struct {
	Temp     float64 `json:"temp"`
	Humidity float64 `json:"humidity"`
	Temp_min float64 `json:"temp_min"`
	Temp_max float64 `json:"temp_max"`
	Pressure float64 `json:"pressure"`
}

type Wind struct {
	Speed float64 `json:"speed"`
}

func (m *Main) kelvinToCelsius() {
	m.Temp = m.Temp - 273.15
	m.Temp_min = m.Temp_min - 273.15
	m.Temp_max = m.Temp_max - 273.15
}

func fetchLiveWeatherData() {
	url := "https://api.openweathermap.org/data/2.5/weather?lat=40.65&lon=-111.85&appid=25a3874cd58d5c0d253a5a7fc33f9ebe"
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var data Data
	err = json.Unmarshal(body, &data)
	if err != nil {
		log.Fatal(err)
	}

	data.Main.kelvinToCelsius()

	dataChannel <- data
}

func predict(w http.ResponseWriter) {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := bigquery.NewClient(ctx, "slc-air-quality")
	if err != nil {
		log.Printf("Failed to create BigQuery client: %v", err)
		http.Error(w, "Failed to create BigQuery client", http.StatusInternalServerError)
		return
	}
	defer client.Close()

	data := <-dataChannel

	predictionQuery := fmt.Sprintf(
		`
		SELECT *
		FROM ML.PREDICT(MODEL %s,
		  (
		  SELECT
		    TIMESTAMP("%v") AS date_timestamp,
		    %v AS temperature,
		    %v AS temp_min,
		    %v AS temp_max,
		    %v AS wind,
		    %v AS humidity,
			%v AS pressure
		  )
		)
		`,
		"`slc-air-quality.meteorology.aqi_model_v2`",
		time.Now().Format("2006-01-02"),
		data.Main.Temp,
		data.Main.Temp_min,
		data.Main.Temp_max,
		data.Wind.Speed,
		data.Main.Humidity,
		data.Main.Pressure)

	query := client.Query(predictionQuery)
	it, err := query.Read(ctx)
	if err != nil {
		log.Printf("Failed to execute query: %v", err)
		http.Error(w, "Failed to execute query", http.StatusInternalServerError)
		return
	}

	var results []map[string]bigquery.Value
	for {
		var row map[string]bigquery.Value
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Printf("Failed to read query results: %v", err)
			http.Error(w, "Failed to read query results", http.StatusInternalServerError)
			return
		}
		results = append(results, row)
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintln(w, fmt.Sprintf("The current AQI is predicted to be %.0f", results[0]["predicted_AQI"]))
}
