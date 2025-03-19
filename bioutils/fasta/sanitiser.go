package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

func main() {
	mode := flag.String("mode", "encode", "Mode: 'encode' or 'decode'")
	inputFile := flag.String("input", "", "Input file path (optional, uses STDIN if not provided)")
	mappingFile := flag.String("mapping", "", "Mapping file path (optional, uses default location if not provided)")
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

	mappingStore, err := NewMappingStore(*mappingFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating mapping store: %v\n", err)
		os.Exit(1)
	}
	defer mappingStore.Close()

	switch *mode {
	case "encode":
		encodeMode(input, mappingStore, *trimLength)
	case "decode":
		decodeMode(input, mappingStore)
	default:
		fmt.Fprintf(os.Stderr, "Invalid mode. Use 'encode' or 'decode'.\n")
		os.Exit(1)
	}
}

func encodeMode(input io.Reader, mappingStore *MappingStore, trimLength int) {
	scanner := bufio.NewScanner(input)
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()

	var currentHeader, currentSequence string
	index := 0

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, ">") {
			if currentHeader != "" {
				processSequence(currentHeader, currentSequence, index, mappingStore, writer, trimLength)
				index++
			}
			currentHeader = line[1:]
			currentSequence = ""
		} else {
			currentSequence += line
		}
	}

	if currentHeader != "" {
		processSequence(currentHeader, currentSequence, index, mappingStore, writer, trimLength)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}
}

func processSequence(header, sequence string, index int, mappingStore *MappingStore, writer *bufio.Writer, trimLength int) {
	hash := sha1.Sum([]byte(sequence))
	trimmedHash := hex.EncodeToString(hash[:])[:trimLength]
	newID := fmt.Sprintf("%d___%s", index+1, trimmedHash) // Changed to triple underscore
	newHeader := fmt.Sprintf(">%s", newID)

	err := mappingStore.StorePair(newID, header)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error storing mapping: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(writer, "%s\n%s\n", newHeader, sequence)
}

func decodeMode(input io.Reader, mappingStore *MappingStore) {
	scanner := bufio.NewScanner(input)
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()

	// Regular expression to match the encoded IDs with triple underscore
	re := regexp.MustCompile(`\d+___[a-f0-9]+`)

	for scanner.Scan() {
		line := scanner.Text()

		// Find all matches in the line
		matches := re.FindAllString(line, -1)

		for _, match := range matches {
			originalID, err := mappingStore.LookupOriginalID(match)
			if err == nil {
				// Replace the encoded ID with the original ID
				line = strings.Replace(line, match, originalID, 1)
			} else {
				// If there's an error, log it but continue processing
				fmt.Fprintf(os.Stderr, "Warning: Could not decode ID %s: %v\n", match, err)
			}
		}

		fmt.Fprintln(writer, line)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}
}
