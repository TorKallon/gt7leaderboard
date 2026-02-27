package cardb

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Car represents a single car entry from the GT7 car database.
type Car struct {
	ID           int
	Name         string
	Manufacturer string
	Category     string
	PPStock      float64 // Performance Points on Comfort Hard tires
}

// Database holds a collection of cars indexed by their ID.
type Database struct {
	cars map[int]*Car
}

// categoryMap normalizes the group column from the CSV to display names.
var categoryMap = map[string]string{
	"1": "Gr.1",
	"2": "Gr.2",
	"3": "Gr.3",
	"4": "Gr.4",
	"B": "Gr.B",
	"X": "Gr.X",
	"N": "N",
}

// normalizeCategory converts a raw category string to its display name.
func normalizeCategory(raw string) string {
	raw = strings.TrimSpace(raw)
	if mapped, ok := categoryMap[raw]; ok {
		return mapped
	}
	return raw
}

// LoadFromReader loads the car database from a reader in CSV format.
// Expected columns: carid,manufacturer,name,group,CH,CM,CS,SH,SM,SS,RH,RM,RS,IM,W,D
func LoadFromReader(r io.Reader) (*Database, error) {
	csvReader := csv.NewReader(r)
	csvReader.TrimLeadingSpace = true

	// Read header.
	header, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}

	// Find column indices.
	colIndex := make(map[string]int, len(header))
	for i, h := range header {
		colIndex[strings.TrimSpace(strings.ToLower(h))] = i
	}

	requiredCols := []string{"carid", "manufacturer", "name", "group", "ch"}
	for _, col := range requiredCols {
		if _, ok := colIndex[col]; !ok {
			return nil, fmt.Errorf("missing required column: %s", col)
		}
	}

	db := &Database{cars: make(map[int]*Car)}

	lineNum := 1 // header was line 1
	for {
		lineNum++
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}

		carIDStr := strings.TrimSpace(record[colIndex["carid"]])
		carID, err := strconv.Atoi(carIDStr)
		if err != nil {
			// Skip rows with non-numeric car IDs (e.g., comments).
			continue
		}

		var ppStock float64
		chStr := strings.TrimSpace(record[colIndex["ch"]])
		if chStr != "" {
			ppStock, err = strconv.ParseFloat(chStr, 64)
			if err != nil {
				ppStock = 0
			}
		}

		car := &Car{
			ID:           carID,
			Name:         strings.TrimSpace(record[colIndex["name"]]),
			Manufacturer: strings.TrimSpace(record[colIndex["manufacturer"]]),
			Category:     normalizeCategory(record[colIndex["group"]]),
			PPStock:      ppStock,
		}

		db.cars[carID] = car
	}

	return db, nil
}

// LoadFromFile loads the car database from a CSV file on disk.
func LoadFromFile(path string) (*Database, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	return LoadFromReader(f)
}

// Lookup returns the car with the given ID, or nil and false if not found.
func (db *Database) Lookup(carID int) (*Car, bool) {
	if db == nil {
		return nil, false
	}
	car, ok := db.cars[carID]
	return car, ok
}

// Count returns the number of cars in the database.
func (db *Database) Count() int {
	if db == nil {
		return 0
	}
	return len(db.cars)
}

// All returns all cars in the database as a slice.
func (db *Database) All() []*Car {
	if db == nil {
		return nil
	}
	result := make([]*Car, 0, len(db.cars))
	for _, car := range db.cars {
		result = append(result, car)
	}
	return result
}

// PPSubBand returns the PP sub-band classification for a given PP value.
//
//	PP < 300  -> "N100-300"
//	300-500   -> "N300-500"
//	500-700   -> "N500-700"
//	>= 700    -> "N700+"
func PPSubBand(pp float64) string {
	switch {
	case pp < 300:
		return "N100-300"
	case pp < 500:
		return "N300-500"
	case pp < 700:
		return "N500-700"
	default:
		return "N700+"
	}
}
