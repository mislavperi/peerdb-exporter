package config

import "github.com/dogmatiq/ferrite"

var PeerDBHost = ferrite.String("PEERDB_HOST", "Database host for underlying database that PeerDB uses").Required()
var PeerDBPort = ferrite.Signed[int16]("PEERDB_PORT", "Database port for underlying database that PeerDB uses").Required()
var PeerDBUsername = ferrite.String("PEERDB_USERNAME", "Database username for underlying database that PeerDB uses").Required()
var PeerDBPassword = ferrite.String("PEERDB_PASSWORD", "Database password for underlying database that PeerDB uses").Required()
var PeerDBDatabase = ferrite.String("PEERDB_DATABASE", "Database where PeerDB metrics are located, usually `peerdb_stats`").WithDefault("peerdb_stats").Required()
