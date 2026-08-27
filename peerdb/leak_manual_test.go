package peerdb_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mislavperi/peerdb-exporter/peerdb"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// scrape returns the number of exported samples whose name starts with peerdb_,
// broken down per metric name.
func scrape(t *testing.T) (int, map[string]int) {
	t.Helper()
	rec := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))

	total := 0
	per := map[string]int{}
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if line == "" || strings.HasPrefix(line, "#") || !strings.HasPrefix(line, "peerdb_") {
			continue
		}
		name := line
		if i := strings.IndexAny(name, "{ "); i >= 0 {
			name = name[:i]
		}
		per[name]++
		total++
	}
	return total, per
}

func alertValue(t *testing.T) string {
	t.Helper()
	rec := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if strings.HasPrefix(line, "peerdb_alerts{") {
			return line[strings.LastIndex(line, " ")+1:]
		}
	}
	return "<absent>"
}

// churn simulates PeerDB advancing: new batch ids, new qrep partitions, new alerts.
func churn(t *testing.T, db *pgxpool.Pool, round int) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < 50; i++ {
		id := round*1000 + i
		for _, flow := range []string{"flow_a", "flow_b"} {
			_, err := db.Exec(ctx, `INSERT INTO peerdb_stats.cdc_batches
				VALUES ($1, $2, 1000, now() - interval '10 seconds', now())`, flow, id)
			if err != nil {
				t.Fatal(err)
			}
			for _, tbl := range []string{"orders", "users"} {
				if _, err := db.Exec(ctx, `INSERT INTO peerdb_stats.cdc_batch_table
					VALUES ($1, $2, $3, 500)`, flow, id, tbl); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.Exec(ctx, `INSERT INTO peerdb_stats.qrep_partitions
				VALUES ($1, $2, 100, now(), CASE WHEN $3 THEN now() ELSE NULL END)`,
				flow, fmt.Sprintf("uuid-%d-%d", round, i), i%2 == 0); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.Exec(ctx, `INSERT INTO peerdb_stats.alerts_v1 (alert_key, alert_level, alert_message, created_timestamp)
			VALUES ('slot_lag', 'critical', $1, now())`,
			fmt.Sprintf("replication slot for peer pg_source is %d bytes behind, round %d", id*7919, round)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSeriesCardinalityStaysBounded(t *testing.T) {
	dsn := os.Getenv("PEERDB_TEST_DSN")
	if dsn == "" {
		t.Skip("PEERDB_TEST_DSN not set")
	}

	db, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	e := peerdb.NewPeerDBExporter(db)

	counts := []int{}
	for round := 1; round <= 5; round++ {
		churn(t, db, round)
		if err := e.CollectMetrics(); err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
		// scrape twice: a second collect with no new rows must not double count
		if err := e.CollectMetrics(); err != nil {
			t.Fatalf("round %d recollect: %v", round, err)
		}
		total, per := scrape(t)
		counts = append(counts, total)
		t.Logf("round %d: %d series alerts_total=%s %v", round, total, alertValue(t), per)
	}

	rec := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if strings.HasPrefix(line, "peerdb_") {
			t.Log(line)
		}
	}

	for i := 1; i < len(counts); i++ {
		if counts[i] != counts[0] {
			t.Errorf("series count not flat across rounds: %v", counts)
			break
		}
	}
}
