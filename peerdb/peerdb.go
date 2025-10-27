package peerdb

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

type PeerDBExporter struct {
	db                 *pgxpool.Pool
	lastReportedTotals map[string]int64

	replicationLag      *prometheus.GaugeVec
	rowsSynced          *prometheus.CounterVec
	rowsSynced24Hours   *prometheus.CounterVec
	syncErrors          *prometheus.CounterVec
	syncThroughput      *prometheus.GaugeVec
	peerStatus          *prometheus.GaugeVec
	lastSyncTime        *prometheus.GaugeVec
	slotSize            *prometheus.GaugeVec
	batchProcessingTime *prometheus.GaugeVec
	qrepPartitionStatus *prometheus.GaugeVec
	pendingRows         *prometheus.GaugeVec
}

func NewPeerDBExporter(pgpool *pgxpool.Pool) *PeerDBExporter {
	exporter := &PeerDBExporter{
		db:                 pgpool,
		lastReportedTotals: make(map[string]int64),
		replicationLag: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "peerdb_replication_lag_seconds",
				Help: "Current replication lag in seconds based on LSN difference",
			},
			[]string{"peer_name", "flow_name"},
		),
		rowsSynced: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "peerdb_rows_synced_total",
				Help: "Total number of rows synced per table",
			},
			[]string{"batch_id", "flow_name", "table_name"},
		),
		rowsSynced24Hours: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "peerdb_rows_synced_24_hours",
				Help: "Total number of rows synced per table",
			},
			[]string{"batch_id", "flow_name", "table_name"},
		),
		syncErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "peerdb_sync_errors_total",
				Help: "Total number of sync errors",
			},
			[]string{"peer_name", "flow_name", "error_type"},
		),
		syncThroughput: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "peerdb_sync_throughput_rows_per_second",
				Help: "Current sync throughput in rows per second",
			},
			[]string{"peer_name", "flow_name"},
		),
		peerStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "peerdb_peer_status",
				Help: "Status of peer connection (1=healthy, 0=unhealthy)",
			},
			[]string{"peer_name"},
		),
		lastSyncTime: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "peerdb_last_sync_timestamp_seconds",
				Help: "Timestamp of last successful sync",
			},
			[]string{"peer_name", "flow_name"},
		),
		slotSize: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "peerdb_replication_slot_size_bytes",
				Help: "Size of replication slot indicating lag",
			},
			[]string{"peer_name"},
		),
		batchProcessingTime: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "peerdb_batch_processing_time_seconds",
				Help: "Time taken to process batches",
			},
			[]string{"peer_name", "flow_name"},
		),
		qrepPartitionStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "peerdb_qrep_partition_status",
				Help: "Status of query replication partitions (1=completed, 0=pending/failed)",
			},
			[]string{"peer_name", "flow_name", "partition_id"},
		),
		pendingRows: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "peerdb_pending_rows_total",
				Help: "Number of rows pending sync per table",
			},
			[]string{"peer_name", "flow_name", "table_name"},
		),
	}

	prometheus.MustRegister(
		exporter.replicationLag,
		exporter.rowsSynced,
		exporter.syncErrors,
		exporter.syncThroughput,
		exporter.peerStatus,
		exporter.lastSyncTime,
		exporter.slotSize,
		exporter.batchProcessingTime,
		exporter.qrepPartitionStatus,
		exporter.pendingRows,
	)

	return exporter
}

func (e *PeerDBExporter) collectReplicationMetrics() error {
	// Use cdc_flows table for LSN-based replication lag
	query := `
		SELECT 
			flow_name,
			latest_lsn_at_source,
			latest_lsn_at_target
		FROM peerdb_stats.cdc_flows
		WHERE flow_name IS NOT NULL
	`

	rows, err := e.db.Query(context.Background(), query)
	if err != nil {
		return fmt.Errorf("failed to query replication metrics from cdc_flows: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var flowName string
		var sourceLSN, targetLSN *int64

		err := rows.Scan(&flowName, &sourceLSN, &targetLSN)
		if err != nil {
			log.Printf("Error scanning replication row: %v", err)
			continue
		}

		// Calculate LSN lag (this is a rough approximation - you may need to adjust based on your LSN format)
		if sourceLSN != nil && targetLSN != nil {
			lag := float64(*sourceLSN - *targetLSN)
			e.replicationLag.WithLabelValues("", flowName).Set(lag)
		}
	}

	return rows.Err()
}

func (e *PeerDBExporter) collectBatchMetrics() error {
	// Use cdc_batches for throughput and processing time
	query := `
		SELECT 
			flow_name,
			rows_in_batch,
			EXTRACT(EPOCH FROM (end_time - start_time)) as processing_time_seconds,
			CASE 
				WHEN end_time IS NOT NULL THEN rows_in_batch / GREATEST(EXTRACT(EPOCH FROM (end_time - start_time)), 1)
				ELSE 0 
			END as throughput
		FROM peerdb_stats.cdc_batches
		WHERE start_time > NOW() - INTERVAL '1 hour'
		AND end_time IS NOT NULL
		ORDER BY end_time DESC
		LIMIT 100
	`

	rows, err := e.db.Query(context.Background(), query)
	if err != nil {
		return fmt.Errorf("failed to query batch metrics: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var flowName string
		var numRows int64
		var processingTime, throughput float64

		err := rows.Scan(&flowName, &numRows, &processingTime, &throughput)
		if err != nil {
			log.Printf("Error scanning batch row: %v", err)
			continue
		}

		e.batchProcessingTime.WithLabelValues("", flowName).Set(processingTime)
		e.syncThroughput.WithLabelValues("", flowName).Set(throughput)
	}

	return rows.Err()
}

func (e *PeerDBExporter) collectTableMetrics() error {
	query := `
SELECT
		cb.batch_id,
    cb.flow_name,
    cbt.destination_table_name,
    COALESCE(SUM(cbt.num_rows), 0) AS total_rows
FROM peerdb_stats.cdc_batch_table cbt
JOIN (
    SELECT batch_id, flow_name, start_time
    FROM (
        SELECT
            batch_id,
            flow_name,
            start_time,
            ROW_NUMBER() OVER (PARTITION BY batch_id ORDER BY start_time DESC) AS rn
        FROM peerdb_stats.cdc_batches
    ) t
    WHERE rn = 1
) cb ON cb.batch_id = cbt.batch_id
GROUP BY cb.batch_id, cb.flow_name, cbt.destination_table_name;
	`

	rows, err := e.db.Query(context.Background(), query)
	if err != nil {
		return fmt.Errorf("failed to query table metrics: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var batchID, flowName, tableName string
		var totalRows int64

		err := rows.Scan(&batchID, &flowName, &tableName, &totalRows)
		if err != nil {
			log.Printf("Error scanning table metrics row: %v", err)
			continue
		}

		key := fmt.Sprintf("%s|%s|%s", batchID, flowName, tableName)
		lastTotal := e.lastReportedTotals[key]
		delta := totalRows - lastTotal

		if delta > 0 {
			e.rowsSynced.WithLabelValues(batchID, flowName, tableName).Add(float64(delta))
			e.lastReportedTotals[key] = totalRows
		}
	}

	return rows.Err()
}

func (e *PeerDBExporter) collectSyncedRows24Hours() error {
	query := `
SELECT
		cb.batch_id,
    cb.flow_name,
    cbt.destination_table_name,
    COALESCE(SUM(cbt.num_rows), 0) AS total_rows
FROM peerdb_stats.cdc_batch_table cbt
JOIN (
    SELECT batch_id, flow_name, start_time
    FROM (
        SELECT
            batch_id,
            flow_name,
            start_time,
            ROW_NUMBER() OVER (PARTITION BY batch_id ORDER BY start_time DESC) AS rn
        FROM peerdb_stats.cdc_batches
    ) t
    WHERE rn = 1
) cb ON cb.batch_id = cbt.batch_id
WHERE cb.start_time > NOW() - INTERVAL '24 hours'
GROUP BY cb.batch_id, cb.flow_name, cbt.destination_table_name;
	`

	rows, err := e.db.Query(context.Background(), query)
	if err != nil {
		return fmt.Errorf("failed to query table metrics: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var batchID, flowName, tableName string
		var totalRows int64

		err := rows.Scan(&batchID, &flowName, &tableName, &totalRows)
		if err != nil {
			log.Printf("Error scanning table metrics row: %v", err)
			continue
		}

		key := fmt.Sprintf("%s|%s|%s", batchID, flowName, tableName)
		lastTotal := e.lastReportedTotals[key]
		delta := totalRows - lastTotal

		if delta > 0 {
			e.rowsSynced.WithLabelValues(batchID, flowName, tableName).Add(float64(delta))
			e.lastReportedTotals[key] = totalRows
		}
	}

	return rows.Err()
}

func (e *PeerDBExporter) collectSlotSizeMetrics() error {
	// Use peer_slot_size for replication slot lag
	query := `
		SELECT 
			peer_name,
			slot_size,
			updated_at
		FROM peerdb_stats.peer_slot_size
		ORDER BY updated_at DESC
	`

	rows, err := e.db.Query(context.Background(), query)
	if err != nil {
		return fmt.Errorf("failed to query slot size metrics: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var peerName string
		var slotSize *int64
		var updatedAt time.Time

		err := rows.Scan(&peerName, &slotSize, &updatedAt)
		if err != nil {
			log.Printf("Error scanning slot size row: %v", err)
			continue
		}

		if slotSize != nil {
			e.slotSize.WithLabelValues(peerName).Set(float64(*slotSize))
		}
	}

	return rows.Err()
}

func (e *PeerDBExporter) collectQRepMetrics() error {
	query := `
		SELECT 
			flow_name,
			partition_uuid,
			rows_in_partition,
			CASE WHEN end_time IS NOT NULL THEN 1 ELSE 0 END as completed_status
		FROM peerdb_stats.qrep_partitions
		WHERE start_time > NOW() - INTERVAL '24 hours'
	`

	rows, err := e.db.Query(context.Background(), query)
	if err != nil {
		log.Printf("Warning: Could not query qrep metrics (may not have query replication flows): %v", err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var flowName, partitionID string
		var numRows int64
		var completedStatus float64

		err := rows.Scan(&flowName, &partitionID, &numRows, &completedStatus)
		if err != nil {
			log.Printf("Error scanning qrep partition row: %v", err)
			continue
		}

		e.qrepPartitionStatus.WithLabelValues("", flowName, partitionID).Set(completedStatus)

		if completedStatus == 0 {
			e.pendingRows.WithLabelValues("", flowName, "qrep_partition").Set(float64(numRows))
		}
	}

	return rows.Err()
}

// Note: PeerDB doesn't seem to have a standard peer status table in the documentation
// You may need to create this based on your specific setup or check connection health
// func (e *PeerDBExporter) collectPeerStatus() error {
// 	// This is a placeholder - you'll need to adjust based on your actual peer health checking mechanism
// 	// For example, you might check if recent batches exist or query connection status
// 	query := `
// 		SELECT DISTINCT
// 			COALESCE(peer_name, 'unknown') as peer_name,
// 			CASE WHEN MAX(updated_at) > NOW() - INTERVAL '5 minutes' THEN 1 ELSE 0 END as health_status
// 		FROM peerdb_stats.peer_slot_size
// 		GROUP BY peer_name
// 		UNION ALL
// 		SELECT
// 			'flow_' || flow_name as peer_name,
// 			CASE WHEN MAX(updated_at) > NOW() - INTERVAL '10 minutes' THEN 1 ELSE 0 END as health_status
// 		FROM peerdb_stats.cdc_flows
// 		GROUP BY flow_name
// 	`
//
// 	rows, err := e.db.Query(context.Background(), query)
// 	if err != nil {
// 		log.Printf("Warning: Could not query peer status: %v", err)
// 		return nil
// 	}
// 	defer rows.Close()
//
// 	for rows.Next() {
// 		var peerName string
// 		var status float64
//
// 		err := rows.Scan(&peerName, &status)
// 		if err != nil {
// 			log.Printf("Error scanning peer status row: %v", err)
// 			continue
// 		}
//
// 		e.peerStatus.WithLabelValues(peerName).Set(status)
// 	}
//
// 	return rows.Err()
// }

// func (e *PeerDBExporter) collectErrorMetrics() error {
// 	// This assumes you have an error tracking mechanism - adjust based on your setup
// 	// PeerDB might store errors in logs or a custom error table
// 	query := `
// 		SELECT
// 			flow_name,
// 			'batch_error' as error_type,
// 			COUNT(*) as error_count
// 		FROM peerdb_stats.cdc_batches
// 		WHERE start_time > NOW() - INTERVAL '1 hour'
// 		AND end_time IS NULL
// 		AND start_time < NOW() - INTERVAL '30 minutes' -- Assume batches taking >30min are errors
// 		GROUP BY flow_name
// 		UNION ALL
// 		SELECT
// 			flow_name,
// 			'partition_retry' as error_type,
// 			COALESCE(SUM(retry_count), 0) as error_count
// 		FROM peerdb_stats.qrep_partitions
// 		WHERE start_time > NOW() - INTERVAL '1 hour'
// 		AND retry_count > 0
// 		GROUP BY flow_name
// 	`
//
// 	rows, err := e.db.Query(context.Background(), query)
// 	if err != nil {
// 		log.Printf("Warning: Could not query error metrics: %v", err)
// 		return nil
// 	}
// 	defer rows.Close()
//
// 	for rows.Next() {
// 		var flowName, errorType string
// 		var errorCount float64
//
// 		err := rows.Scan(&flowName, &errorType, &errorCount)
// 		if err != nil {
// 			log.Printf("Error scanning error metrics row: %v", err)
// 			continue
// 		}
//
// 		e.syncErrors.WithLabelValues("", flowName, errorType).Add(errorCount)
// 	}
//
// 	return rows.Err()
// }

func (e *PeerDBExporter) CollectMetrics() error {
	if err := e.collectReplicationMetrics(); err != nil {
		return fmt.Errorf("failed to collect replication metrics: %w", err)
	}

	if err := e.collectBatchMetrics(); err != nil {
		return fmt.Errorf("failed to collect batch metrics: %w", err)
	}

	if err := e.collectTableMetrics(); err != nil {
		return fmt.Errorf("failed to collect table metrics: %w", err)
	}

	if err := e.collectSyncedRows24Hours(); err != nil {
		return fmt.Errorf("failed to collect synced rows in 24 hours metrics: %w", err)
	}

	if err := e.collectSlotSizeMetrics(); err != nil {
		return fmt.Errorf("failed to collect slot size metrics: %w", err)
	}

	if err := e.collectQRepMetrics(); err != nil {
		log.Printf("Warning: Could not collect QRep metrics: %v", err)
	}

	// if err := e.collectPeerStatus(); err != nil {
	// 	log.Printf("Warning: Could not collect peer status: %v", err)
	// }

	// if err := e.collectErrorMetrics(); err != nil {
	// 	log.Printf("Warning: Could not collect error metrics: %v", err)
	// }

	return nil
}

func (e *PeerDBExporter) StartMetricsCollection(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		if err := e.CollectMetrics(); err != nil {
			log.Printf("Error collecting initial metrics: %v", err)
		}

		for range ticker.C {
			if err := e.CollectMetrics(); err != nil {
				log.Printf("Error collecting metrics: %v", err)
			} else {
				log.Printf("Successfully collected metrics at %v", time.Now())
			}
		}
	}()
}
