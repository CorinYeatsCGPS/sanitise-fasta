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

const (
	idFormat      = "%d_PW_%s"
	idRegexFormat = `\d+_PW_[a-f0-9]+`
)

func main() {
	storeLocation := flag.String("store", "", "Location to store mapping data (optional, uses current directory if not provided)")
	trimLength := flag.Int("trim", 40, "Number of characters to keep from the SHA1 checksum (optional, uses 40 if not provided). Maximum is 40.")
	csvMode := flag.Bool("csv", false, "Enable CSV mode for decoding (puts original IDs in quotes)")

	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: %s [options] [encode|decode] <input_file>\n\n", os.Args[0])
		_, _ = fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		_, _ = fmt.Fprintf(os.Stderr, "\nExample usage:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  Encode: %s encode input.fasta > output.fasta\n", os.Args[0])
		_, _ = fmt.Fprintf(os.Stderr, "  Decode: %s decode input.txt > output.txt\n", os.Args[0])
		_, _ = fmt.Fprintf(os.Stderr, "  Decode CSV: %s -csv decode input.csv > output.csv\n", os.Args[0])
		_, _ = fmt.Fprintf(os.Stderr, "  Use '-' as input_file to read from STDIN\n")
	}

	flag.Parse()

	args := flag.Args()
	if len(args) != 2 || (args[0] != "encode" && args[0] != "decode") {
		flag.Usage()
		os.Exit(1)
	}

	mode := args[0]
	inputFile := args[1]

	if *trimLength < 1 || *trimLength > 40 {
		_, _ = fmt.Fprintf(os.Stderr, "Error: trim value must be between 1 and 40\n")
		os.Exit(1)
	}

	if mode == "encode" && *csvMode {
		_, _ = fmt.Fprintf(os.Stderr, "Error: CSV mode (-csv) is only applicable for decode mode\n")
		os.Exit(1)
	}

	var input io.Reader
	if inputFile == "-" {
		input = os.Stdin
	} else {
		file, err := os.Open(inputFile)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error opening input file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()
		input = file
	}

	switch mode {
	case "encode":
		mappingStore, err := NewMappingStore(*storeLocation, false)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error creating mapping store: %v\n", err)
			os.Exit(1)
		}
		defer func() {
			if err := mappingStore.Close(); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error closing mapping store: %v\n", err)
			}
		}()
		if err := encodeMode(input, mappingStore, *trimLength); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error in encode mode: %v\n", err)
			os.Exit(1)
		}
	case "decode":
		mappingStore, err := NewMappingStore(*storeLocation, true)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error creating mapping store: %v\n", err)
			os.Exit(1)
		}
		defer mappingStore.Close()
		if err := decodeMode(input, mappingStore, *csvMode); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error in decode mode: %v\n", err)
			os.Exit(1)
		}
	}
}

func encodeMode(input io.Reader, mappingStore *MappingStore, trimLength int) error {
	scanner := bufio.NewScanner(input)
	writer := bufio.NewWriter(os.Stdout)
	defer func() {
		if err := writer.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "Error flushing writer: %v\n", err)
		}
	}()

	// Increase the buffer size to handle larger lines
	const maxCapacity = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	var currentHeader, currentSequence string
	index := 0

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, ">") {
			if currentHeader != "" {
				if err := processSequence(currentHeader, currentSequence, index, writer, trimLength, mappingStore); err != nil {
					return fmt.Errorf("error processing sequence: %v", err)
				}
				index++
			}
			currentHeader = line[1:]
			currentSequence = ""
		} else {
			currentSequence += line
		}
	}

	if currentHeader != "" {
		if err := processSequence(currentHeader, currentSequence, index, writer, trimLength, mappingStore); err != nil {
			return fmt.Errorf("error processing sequence: %v", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input: %v", err)
	}

	// Finalize the database (commit transaction, create index, analyze)
	if err := mappingStore.Finalise(); err != nil {
		return fmt.Errorf("error finalizing database: %v", err)
	}

	if _, err := fmt.Fprintf(os.Stderr, "Encoding completed. Database optimized.\n"); err != nil {
		return fmt.Errorf("error writing completion message: %v", err)
	}

	return nil
}

func processSequence(header, sequence string, index int, writer *bufio.Writer, trimLength int, mappingStore *MappingStore) error {
	hash := sha1.Sum([]byte(sequence))
	trimmedHash := hex.EncodeToString(hash[:])[:trimLength]
	newID := fmt.Sprintf(idFormat, index+1, trimmedHash)
	newHeader := fmt.Sprintf(">%s", newID)

	// Store the mapping directly
	if err := mappingStore.StorePair(newID, header); err != nil {
		return fmt.Errorf("error storing mapping: %v", err)
	}

	_, err := fmt.Fprintf(writer, "%s\n%s\n", newHeader, sequence)
	if err != nil {
		return fmt.Errorf("error writing sequence: %v", err)
	}

	return nil
}

func decodeMode(input io.Reader, mappingStore *MappingStore, csvMode bool) error {
	scanner := bufio.NewScanner(input)
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()

	// Increase the buffer size to handle larger lines
	const maxCapacity = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	// Regular expression to match the encoded IDs
	re := regexp.MustCompile(idRegexFormat)

	lineNum := 0
	for scanner.Scan() {
		line := scanner.Text()

		// Find all matches in the line
		matches := re.FindAllString(line, -1)

		if len(matches) > 0 {
			// Create a map of replacements
			replacements := make(map[string]string)
			for _, match := range matches {
				originalID, err := mappingStore.LookupOriginalID(match)
				if err == nil {
					if csvMode {
						// Escape any existing double quotes in the original ID
						originalID = strings.ReplaceAll(originalID, `"`, `""`)
						// Wrap the original ID in double quotes
						originalID = fmt.Sprintf(`"%s"`, originalID)
					}
					replacements[match] = originalID
				} else {
					fmt.Fprintf(os.Stderr, "Warning: Could not decode ID %s: %v\n", match, err)
				}
			}

			// Perform a single pass replacement
			for oldID, newID := range replacements {
				line = strings.ReplaceAll(line, oldID, newID)
			}
		}

		if _, err := fmt.Fprintln(writer, line); err != nil {
			return fmt.Errorf("error writing output at line %d: %v", lineNum, err)
		}

		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input: %v", err)
	}

	return nil
}
