package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MaintenanceHandler struct {
	db      *pgxpool.Pool
	minio   *minio.Client
	appEnv  string
	buckets []string
}

func NewMaintenanceHandler(
	db *pgxpool.Pool,
	appEnv string,
	minioEndpoint string,
	minioAccessKey string,
	minioSecretKey string,
	minioUseSSL bool,
	buckets []string,
) (*MaintenanceHandler, error) {
	client, err := minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAccessKey, minioSecretKey, ""),
		Secure: minioUseSSL,
	})
	if err != nil {
		return nil, err
	}

	return &MaintenanceHandler{
		db:      db,
		minio:   client,
		appEnv:  appEnv,
		buckets: uniqueNonEmptyStrings(buckets),
	}, nil
}

func (h *MaintenanceHandler) ResetDB(c *gin.Context) {
	if !h.ensureDev(c) {
		return
	}

	count, err := truncateAllPublicTables(c.Request.Context(), h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reset db"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "database cleaned",
		"truncatedTables": count,
	})
}

func (h *MaintenanceHandler) ResetMinIO(c *gin.Context) {
	if !h.ensureDev(c) {
		return
	}

	removed, err := h.removeAllObjectsFromBuckets(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reset minio"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "minio cleaned",
		"removedObjects": removed,
		"buckets":        h.buckets,
	})
}

func (h *MaintenanceHandler) ensureDev(c *gin.Context) bool {
	if strings.ToLower(strings.TrimSpace(h.appEnv)) != "dev" {
		c.JSON(http.StatusForbidden, gin.H{"error": "available only in dev environment"})
		return false
	}
	return true
}

func truncateAllPublicTables(ctx context.Context, db *pgxpool.Pool) (int, error) {
	rows, err := db.Query(ctx, `
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		  AND tablename NOT IN ('schema_migrations', 'goose_db_version')
		ORDER BY tablename
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	tableNames := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return 0, err
		}
		tableNames = append(tableNames, pgx.Identifier{"public", name}.Sanitize())
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	if len(tableNames) == 0 {
		return 0, nil
	}

	query := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", strings.Join(tableNames, ", "))
	if _, err := db.Exec(ctx, query); err != nil {
		return 0, err
	}

	return len(tableNames), nil
}

func (h *MaintenanceHandler) removeAllObjectsFromBuckets(ctx context.Context) (int, error) {
	totalRemoved := 0

	for _, bucket := range h.buckets {
		exists, err := h.minio.BucketExists(ctx, bucket)
		if err != nil {
			return totalRemoved, err
		}
		if !exists {
			continue
		}

		for object := range h.minio.ListObjects(ctx, bucket, minio.ListObjectsOptions{
			Recursive: true,
		}) {
			if object.Err != nil {
				return totalRemoved, object.Err
			}

			if err := h.minio.RemoveObject(ctx, bucket, object.Key, minio.RemoveObjectOptions{}); err != nil {
				return totalRemoved, err
			}
			totalRemoved++
		}
	}

	return totalRemoved, nil
}

func uniqueNonEmptyStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))

	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}

	return out
}
