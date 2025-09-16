package main

import (
	"net/http"
	"time"

	"github.com/dogmatiq/ferrite"
	"github.com/mislavperi/peerdb-exporter/config"
	"github.com/mislavperi/peerdb-exporter/infrastructure/postgres"
	"github.com/mislavperi/peerdb-exporter/peerdb"
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

	dbConnection, err := postgres.NewDBConnection(databaseDetails)
	if err != nil {
		panic(err)
	}

	peerDBExporter := peerdb.NewPeerDBExporter(dbConnection)

	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		peerDBExporter.StartMetricsCollection(time.Second * 5)
	})

	if err := http.ListenAndServe(":8080", mux); err != nil {
		panic(err)
	}
}
