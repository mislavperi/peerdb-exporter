package config

import "github.com/dogmatiq/ferrite"

var Host = ferrite.String("HOST", "Database host for underlying database that PeerDB uses").Required()
var Port = ferrite.Signed[int16]("PORT", "Database port for underlying database that PeerDB uses").Required()
var Username = ferrite.String("USERNAME", "Database username for underlying database that PeerDB uses").Required()
var Password = ferrite.String("PASSWORD", "Database password for underlying database that PeerDB uses").Required()
var Database = ferrite.String("DATABASE", "Database where PeerDB metrics are located, usually `peerdb_stats`").WithDefault("peerdb_stats").Required()
