package main

import (
	"log"
	"net/http"
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
		Host:     config.PeerDBHost.Value(),
		Port:     config.PeerDBPort.Value(),
		Username: config.PeerDBUsername.Value(),
		Password: config.PeerDBPassword.Value(),
		Database: config.PeerDBDatabase.Value(),
	}

	log.Printf("Connecting to database: %s:%d/%s", databaseDetails.Host, databaseDetails.Port, databaseDetails.Database)

	dbConnection, err := postgres.NewDBConnection(databaseDetails)
	if err != nil {
		log.Printf("Failed to connect to database: %v", err)
		panic(err)
	}

	log.Printf("Successfully connected to database")
	peerDBExporter := peerdb.NewPeerDBExporter(dbConnection)

	// Start collecting metrics in background
	peerDBExporter.StartMetricsCollection(time.Second * 30)

	mux := http.NewServeMux()
	
	// Serve Prometheus metrics using the standard handler
	mux.Handle("/metrics", promhttp.Handler())

	log.Printf("Starting PeerDB Exporter on port 8001")
	log.Printf("Metrics available at: http://localhost:8001/metrics")

	if err := http.ListenAndServe(":8001", mux); err != nil {
		panic(err)
	}
}
