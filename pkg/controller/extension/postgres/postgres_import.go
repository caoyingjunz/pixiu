package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	apierrors "github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

const (
	maxImportStatements  = 200000
	maxImportResultItems = 500
	pgImportTimeout      = 15 * time.Minute
)

// ImportSQL executes an uploaded SQL script in one database session and stops at the first error.
func (c *controller) ImportSQL(ctx context.Context, id int64, database string, script io.Reader) (*types.PostgresImportResult, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := pgCheckIdent("database", database); err != nil {
		return nil, err
	}
	statements, err := readPgImport(script, types.PostgresImportMaxBytes, maxImportStatements)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, pgImportTimeout)
	defer cancel()
	db, _, err := c.connTo(ctx, id, database)
	if err != nil {
		return nil, err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, wrapPgErr(err)
	}
	defer func() {
		// Do not return imported session settings, temp tables or open transactions to the pool.
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		_ = conn.Close()
	}()
	return runPgImport(ctx, statements, func(ctx context.Context, statement string) (sql.Result, error) {
		return conn.ExecContext(ctx, statement)
	}), nil
}

func readPgImport(script io.Reader, maxBytes, maxStatements int) ([]pgStatement, error) {
	data, err := io.ReadAll(io.LimitReader(script, int64(maxBytes)+1))
	var tooLarge *http.MaxBytesError
	if len(data) > maxBytes || errors.As(err, &tooLarge) {
		return nil, apierrors.NewError(fmt.Errorf("sql file exceeds limit %d bytes", maxBytes), http.StatusRequestEntityTooLarge)
	}
	if err != nil {
		return nil, apierrors.NewError(fmt.Errorf("failed to read SQL script: %w", err), http.StatusBadRequest)
	}
	statements := pgSplitStatements(strings.TrimPrefix(string(data), "\ufeff"), maxStatements)
	if len(statements) == 0 {
		return nil, apierrors.NewError(fmt.Errorf("no executable statement found"), http.StatusBadRequest)
	}
	if len(statements) > maxStatements {
		return nil, apierrors.NewError(fmt.Errorf("too many statements: limit %d", maxStatements), http.StatusBadRequest)
	}
	return statements, nil
}

func runPgImport(ctx context.Context, statements []pgStatement, exec func(context.Context, string) (sql.Result, error)) *types.PostgresImportResult {
	batch := &types.PostgresImportResult{PostgresBatchResult: types.PostgresBatchResult{
		Items: make([]types.PostgresBatchItem, 0, min(len(statements), 2*maxImportResultItems)),
		Total: len(statements),
	}}
	for i, st := range statements {
		item := types.PostgresBatchItem{Index: i + 1, StartLine: st.StartLine}
		start := time.Now()
		result, err := exec(ctx, st.Text)
		if err != nil {
			item.Error = pgTruncateRunes(wrapPgErr(err).Error())
			batch.StoppedAt = item.Index
		} else {
			affected, _ := result.RowsAffected()
			item.Ok = true
			item.Result = &types.PostgresQueryResult{
				Affected: affected, Duration: time.Since(start).Milliseconds(),
				Statement: strings.ToLower(pgFirstKeyword(st.Text)),
			}
		}
		batch.Executed = item.Index
		batch.Items = append(batch.Items, item)
		if len(batch.Items) == 2*maxImportResultItems {
			copy(batch.Items, batch.Items[maxImportResultItems:])
			batch.Items = batch.Items[:maxImportResultItems]
		}
		if err != nil {
			break
		}
	}
	if len(batch.Items) > maxImportResultItems {
		batch.Items = batch.Items[len(batch.Items)-maxImportResultItems:]
	}
	batch.ItemsOmitted = batch.Executed - len(batch.Items)
	return batch
}
