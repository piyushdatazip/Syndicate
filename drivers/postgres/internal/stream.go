package driver

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/piyushsingariya/shift/drivers/base"
	"github.com/piyushsingariya/shift/protocol"
	"github.com/piyushsingariya/shift/safego"
	"github.com/piyushsingariya/shift/types"
	"github.com/piyushsingariya/shift/typing"
)

type pgStream struct {
	protocol.Stream
}

const (
	readRecordsFullRefresh             = `SELECT * FROM "%s"."%s" OFFSET %d LIMIT %d`
	readRecordsIncrementalWithState    = `SELECT * FROM "%s"."%s" where "%s">= $1 ORDER BY "%s" ASC NULLS FIRST OFFSET %d LIMIT %d`
	readRecordsIncrementalWithoutState = `SELECT * FROM "%s"."%s" ORDER BY "%s" ASC NULLS FIRST OFFSET %d LIMIT %d`
)

func (p *pgStream) readFullRefresh(client *sqlx.DB, channel chan<- types.Record) error {
	offset := int64(0)
	limit := p.BatchSize()

	for {
		statement := fmt.Sprintf(readRecordsFullRefresh, p.Namespace(), p.Name(), offset*limit, limit)

		// Execute the query
		rows, err := client.Queryx(statement)
		if err != nil {
			return typing.SQLError(typing.ReadTableError, err, fmt.Sprintf("failed to read after offset[%d] limit[%d]", offset*limit, limit), &typing.ErrorPayload{
				Table:     p.Name(),
				Schema:    p.Namespace(),
				Statement: statement,
			})
		}

		paginationFinished := true

		// Fetch rows and populate the result
		for rows.Next() {
			paginationFinished = false

			// Create a map to hold column names and values
			record := make(types.RecordData)

			// Scan the row into the map
			err := rows.MapScan(record)
			if err != nil {
				return fmt.Errorf("failed to mapScan record data: %s", err)
			}

			// insert record
			if !safego.Insert(channel, base.ReformatRecord(p, record)) {
				// channel was closed
				return nil
			}
		}

		// Check for any errors during row iteration
		err = rows.Err()
		if err != nil {
			return fmt.Errorf("failed to mapScan record data: %s", err)
		}

		// records finished
		if paginationFinished {
			break
		}

		// increase offset
		offset += 1
		rows.Close()
	}
	return nil
}

func (p *pgStream) readIncremental(client *sqlx.DB, channel chan<- types.Record) error {
	offset := int64(0)
	limit := p.BatchSize()
	initialStateAtStart := p.GetState()
	cursorDataType := p.JSONSchema().Properties[p.Cursor()].Type

	var extract func() (*sqlx.Rows, error) = func() (*sqlx.Rows, error) {
		if initialStateAtStart != nil {
			statement := fmt.Sprintf(readRecordsIncrementalWithState, p.Namespace, p.Name, p.Cursor(), p.Cursor(), offset*limit, limit)
			// Execute the query
			return client.Queryx(statement, initialStateAtStart)
		}
		statement := fmt.Sprintf(readRecordsIncrementalWithoutState, p.Namespace, p.Name, p.Cursor(), offset*limit, limit)
		// Execute the query
		return client.Queryx(statement)
	}

	for {
		// extract rows
		rows, err := extract()
		if err != nil {
			return typing.SQLError(typing.ReadTableError, err, fmt.Sprintf("failed to read after offset[%d] limit[%d]", offset*limit, limit), &typing.ErrorPayload{
				Table:  p.Name(),
				Schema: p.Namespace(),
			})
		}

		paginationFinished := true

		// Fetch rows and populate the result
		for rows.Next() {
			paginationFinished = false

			// Create a map to hold column names and values
			record := make(types.RecordData)

			// Scan the row into the map
			err := rows.MapScan(record)
			if err != nil {
				return fmt.Errorf("failed to mapScan record data: %s", err)
			}

			if cursorVal, found := record[p.Cursor()]; found && cursorVal != nil {
				// compare if not nil
				if p.GetState() != nil {
					state, err := typing.MaximumOnDataType(cursorDataType, p.GetState(), cursorVal)
					if err != nil {
						return err
					}

					p.SetState(state)
				} else {
					// directly update
					p.SetState(cursorVal)
				}
			}

			// insert record
			channel <- base.ReformatRecord(p, record)
		}

		// Check for any errors during row iteration
		err = rows.Err()
		if err != nil {
			return fmt.Errorf("failed to mapScan record data: %s", err)
		}

		// records finished
		if paginationFinished {
			break
		}

		// increase offset
		offset += 1
		rows.Close()
	}

	return nil
}