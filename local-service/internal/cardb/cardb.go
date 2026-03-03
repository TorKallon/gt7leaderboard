package cardb

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

// Sources holds readers for the four data files used to build the car database.
type Sources struct {
	Cars      io.Reader // cars.csv: ID, ShortName, Maker
	Makers    io.Reader // maker.csv: ID, Name, Country
	CarGroups io.Reader // cargrp.csv: ID, Group
	StockPerf io.Reader // data-stock-perf.csv: carid, manufacturer, name, group, CH, ...
}

// rawCar is an intermediate type from parsing cars.csv.
type rawCar struct {
	ID        int
	ShortName string
	MakerID   int
}

// stockPerfEntry is an intermediate type from parsing data-stock-perf.csv.
type stockPerfEntry struct {
	Name     string
	Category string
	PPStock  float64
}

// parseCarsCSV parses the cars.csv file (ID, ShortName, Maker).
func parseCarsCSV(r io.Reader) (map[int]rawCar, error) {
	csvReader := csv.NewReader(r)
	csvReader.TrimLeadingSpace = true

	header, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("read cars.csv header: %w", err)
	}

	colIndex := makeColIndex(header)
	for _, col := range []string{"id", "shortname", "maker"} {
		if _, ok := colIndex[col]; !ok {
			return nil, fmt.Errorf("cars.csv missing required column: %s", col)
		}
	}

	result := make(map[int]rawCar)
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("cars.csv: %w", err)
		}

		id, err := strconv.Atoi(strings.TrimSpace(record[colIndex["id"]]))
		if err != nil {
			continue
		}
		makerID, _ := strconv.Atoi(strings.TrimSpace(record[colIndex["maker"]]))

		result[id] = rawCar{
			ID:        id,
			ShortName: strings.TrimSpace(record[colIndex["shortname"]]),
			MakerID:   makerID,
		}
	}
	return result, nil
}

// parseMakerCSV parses the maker.csv file (ID, Name, Country).
func parseMakerCSV(r io.Reader) (map[int]string, error) {
	csvReader := csv.NewReader(r)
	csvReader.TrimLeadingSpace = true

	header, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("read maker.csv header: %w", err)
	}

	colIndex := makeColIndex(header)
	for _, col := range []string{"id", "name"} {
		if _, ok := colIndex[col]; !ok {
			return nil, fmt.Errorf("maker.csv missing required column: %s", col)
		}
	}

	result := make(map[int]string)
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("maker.csv: %w", err)
		}

		id, err := strconv.Atoi(strings.TrimSpace(record[colIndex["id"]]))
		if err != nil {
			continue
		}
		result[id] = strings.TrimSpace(record[colIndex["name"]])
	}
	return result, nil
}

// parseCarGrpCSV parses the cargrp.csv file (ID, Group).
func parseCarGrpCSV(r io.Reader) (map[int]string, error) {
	csvReader := csv.NewReader(r)
	csvReader.TrimLeadingSpace = true

	header, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("read cargrp.csv header: %w", err)
	}

	colIndex := makeColIndex(header)
	for _, col := range []string{"id", "group"} {
		if _, ok := colIndex[col]; !ok {
			return nil, fmt.Errorf("cargrp.csv missing required column: %s", col)
		}
	}

	result := make(map[int]string)
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("cargrp.csv: %w", err)
		}

		id, err := strconv.Atoi(strings.TrimSpace(record[colIndex["id"]]))
		if err != nil {
			continue
		}
		result[id] = strings.TrimSpace(record[colIndex["group"]])
	}
	return result, nil
}

// parseStockPerfCSV parses data-stock-perf.csv (carid, manufacturer, name, group, CH, ...).
func parseStockPerfCSV(r io.Reader) (map[int]stockPerfEntry, error) {
	csvReader := csv.NewReader(r)
	csvReader.TrimLeadingSpace = true

	header, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("read stock-perf.csv header: %w", err)
	}

	colIndex := makeColIndex(header)
	for _, col := range []string{"carid", "name", "group", "ch"} {
		if _, ok := colIndex[col]; !ok {
			return nil, fmt.Errorf("stock-perf.csv missing required column: %s", col)
		}
	}

	result := make(map[int]stockPerfEntry)
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("stock-perf.csv: %w", err)
		}

		id, err := strconv.Atoi(strings.TrimSpace(record[colIndex["carid"]]))
		if err != nil {
			continue
		}

		var pp float64
		chStr := strings.TrimSpace(record[colIndex["ch"]])
		if chStr != "" {
			pp, _ = strconv.ParseFloat(chStr, 64)
		}

		result[id] = stockPerfEntry{
			Name:     strings.TrimSpace(record[colIndex["name"]]),
			Category: strings.TrimSpace(record[colIndex["group"]]),
			PPStock:  pp,
		}
	}
	return result, nil
}

// LoadFromSources builds a car database by merging data from four CSV sources.
// cars.csv is the authoritative roster. maker.csv provides manufacturer names.
// cargrp.csv provides category codes. data-stock-perf.csv provides richer names and PP values.
func LoadFromSources(s Sources) (*Database, error) {
	cars, err := parseCarsCSV(s.Cars)
	if err != nil {
		return nil, fmt.Errorf("parsing cars: %w", err)
	}

	makers, err := parseMakerCSV(s.Makers)
	if err != nil {
		return nil, fmt.Errorf("parsing makers: %w", err)
	}

	groups, err := parseCarGrpCSV(s.CarGroups)
	if err != nil {
		return nil, fmt.Errorf("parsing car groups: %w", err)
	}

	var perf map[int]stockPerfEntry
	if s.StockPerf != nil {
		perf, err = parseStockPerfCSV(s.StockPerf)
		if err != nil {
			return nil, fmt.Errorf("parsing stock perf: %w", err)
		}
	}

	db := &Database{cars: make(map[int]*Car, len(cars))}

	for id, raw := range cars {
		car := &Car{ID: id}

		// Manufacturer from maker.csv
		if name, ok := makers[raw.MakerID]; ok {
			car.Manufacturer = name
		}

		// Category from cargrp.csv, default to "N" if absent
		if grp, ok := groups[id]; ok {
			car.Category = normalizeCategory(grp)
		} else {
			car.Category = "N"
		}

		// Prefer richer name/PP from stock-perf when available
		if sp, ok := perf[id]; ok {
			car.Name = sp.Name
			car.PPStock = sp.PPStock
			// Also prefer stock-perf category if available
			if sp.Category != "" {
				car.Category = normalizeCategory(sp.Category)
			}
		} else {
			car.Name = raw.ShortName
		}

		db.cars[id] = car
	}

	return db, nil
}

// LoadFromDir loads the car database from four CSV files in a directory.
func LoadFromDir(dir string) (*Database, error) {
	openFile := func(name string) (*os.File, error) {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", name, err)
		}
		return f, nil
	}

	carsFile, err := openFile("cars.csv")
	if err != nil {
		return nil, err
	}
	defer carsFile.Close()

	makerFile, err := openFile("maker.csv")
	if err != nil {
		return nil, err
	}
	defer makerFile.Close()

	cargrpFile, err := openFile("cargrp.csv")
	if err != nil {
		return nil, err
	}
	defer cargrpFile.Close()

	// stock-perf is optional (enrichment data)
	var stockPerfReader io.Reader
	stockPerfFile, err := openFile("data-stock-perf.csv")
	if err == nil {
		defer stockPerfFile.Close()
		stockPerfReader = stockPerfFile
	}

	return LoadFromSources(Sources{
		Cars:      carsFile,
		Makers:    makerFile,
		CarGroups: cargrpFile,
		StockPerf: stockPerfReader,
	})
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

// makeColIndex builds a lowercase column name -> index map from a CSV header.
func makeColIndex(header []string) map[string]int {
	m := make(map[string]int, len(header))
	for i, h := range header {
		m[strings.TrimSpace(strings.ToLower(h))] = i
	}
	return m
}
