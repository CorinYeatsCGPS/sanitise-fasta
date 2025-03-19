package main

import (
	"bufio"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	mode := flag.String("mode", "encode", "Mode: 'encode' or 'decode'")
	inputFile := flag.String("input", "", "Input file path (optional, uses STDIN if not provided)")
	mappingFile := flag.String("mapping", "mapping.txt", "Mapping file path")
	trimLength := flag.Int("trim", 40, "Number of characters to keep from the SHA1 checksum (max 40)")
	flag.Parse()

	if *trimLength < 1 || *trimLength > 40 {
		fmt.Fprintf(os.Stderr, "Error: trim value must be between 1 and 40\n")
		os.Exit(1)
	}

	var input io.Reader
	if *inputFile != "" {
		file, err := os.Open(*inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening input file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()
		input = file
	} else {
		input = os.Stdin
	}

	switch *mode {
	case "encode":
		encodeMode(input, *mappingFile, *trimLength)
	case "decode":
		decodeMode(input, *mappingFile)
	default:
		fmt.Fprintf(os.Stderr, "Invalid mode. Use 'encode' or 'decode'.\n")
		os.Exit(1)
	}
}

func encodeMode(input io.Reader, mappingFile string, trimLength int) {
	scanner := bufio.NewScanner(input)
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()

	mapping := make(map[string]string)
	var currentHeader, currentSequence string
	index := 0

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, ">") {
			if currentHeader != "" {
				processSequence(currentHeader, currentSequence, index, mapping, writer, trimLength)
				index++
			}
			currentHeader = line[1:]
			currentSequence = ""
		} else {
			currentSequence += line
		}
	}

	if currentHeader != "" {
		processSequence(currentHeader, currentSequence, index, mapping, writer, trimLength)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}

	writeMappingFile(mappingFile, mapping)
}

func processSequence(header, sequence string, index int, mapping map[string]string, writer *bufio.Writer, trimLength int) {
	hash := sha1.Sum([]byte(sequence))
	trimmedHash := hex.EncodeToString(hash[:])[:trimLength]
	newID := fmt.Sprintf("%d_%s", index+1, trimmedHash)
	newHeader := fmt.Sprintf(">%s", newID)

	// Store the mapping without the '>' character
	mapping[newID] = header

	// Write the new header (with '>') and sequence to the output
	fmt.Fprintf(writer, "%s\n%s\n", newHeader, sequence)
}

func writeMappingFile(filename string, mapping map[string]string) {
	db, err := sql.Open("sqlite3", filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS mapping (
        new_id TEXT PRIMARY KEY,
        original_header TEXT
    )`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating table: %v\n", err)
		os.Exit(1)
	}

	tx, err := db.Begin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting transaction: %v\n", err)
		os.Exit(1)
	}

	stmt, err := tx.Prepare("INSERT INTO mapping (new_id, original_header) VALUES (?, ?)")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error preparing statement: %v\n", err)
		os.Exit(1)
	}
	defer stmt.Close()

	for newID, originalHeader := range mapping {
		_, err = stmt.Exec(newID, originalHeader)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error inserting mapping: %v\n", err)
			tx.Rollback()
			os.Exit(1)
		}
	}

	err = tx.Commit()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error committing transaction: %v\n", err)
		os.Exit(1)
	}
}

func decodeMode(input io.Reader, mappingFile string) {
	mapping := loadMappingFile(mappingFile)
	scanner := bufio.NewScanner(input)
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()

	for scanner.Scan() {
		line := scanner.Text()
		for newID, originalID := range mapping {
			line = strings.ReplaceAll(line, newID, originalID)
		}
		fmt.Fprintln(writer, line)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}
}

func loadMappingFile(filename string) map[string]string {
	db, err := sql.Open("sqlite3", filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	mapping := make(map[string]string)
	rows, err := db.Query("SELECT new_id, original_header FROM mapping")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying database: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	for rows.Next() {
		var newID, originalHeader string
		err := rows.Scan(&newID, &originalHeader)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning row: %v\n", err)
			os.Exit(1)
		}
		mapping[newID] = originalHeader
	}

	if err := rows.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error iterating rows: %v\n", err)
		os.Exit(1)
	}

	return mapping
}
