package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/dogmatiq/ferrite"
	"github.com/mislavperi/peerdb-exporter/config"
	"github.com/mislavperi/peerdb-exporter/infrastructure/postgres"
	"github.com/mislavperi/peerdb-exporter/peerdb"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	ferrite.Init()
	databaseDetails := postgres.PGConnectionDetails{
		Host:     config.Host.Value(),
		Port:     config.Port.Value(),
		Username: config.Username.Value(),
		Password: config.Password.Value(),
		Database: config.Database.Value(),
	}

	log.Printf("Connecting to database: %s:%d/%s", databaseDetails.Host, databaseDetails.Port, databaseDetails.Database)

	dbConnection, err := postgres.NewDBConnection(databaseDetails)
	if err != nil {
		log.Printf("Failed to connect to database: %v", err)
		panic(err)
	}

	log.Printf("Successfully connected to database")
	peerDBExporter := peerdb.NewPeerDBExporter(dbConnection)

	peerDBExporter.StartMetricsCollection(time.Second * 30)

	mux := http.NewServeMux()

	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/debug/", http.DefaultServeMux)
	mux.HandleFunc("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	})

	log.Printf("Starting PeerDB Exporter on port 8001")
	log.Printf("Metrics available at: http://localhost:8001/metrics")

	if err := http.ListenAndServe(":8001", mux); err != nil {
		panic(err)
	}
}
