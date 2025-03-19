package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
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
	newHeader := fmt.Sprintf(">%d_%s", index+1, trimmedHash)
	mapping[newHeader[1:]] = header
	fmt.Fprintf(writer, "%s\n%s\n", newHeader, sequence)
}

func writeMappingFile(filename string, mapping map[string]string) {
	file, err := os.Create(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating mapping file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	for newID, originalID := range mapping {
		fmt.Fprintf(writer, "%s\t%s\n", newID, originalID)
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
	file, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening mapping file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	mapping := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "\t")
		if len(parts) == 2 {
			mapping[parts[0]] = parts[1]
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading mapping file: %v\n", err)
		os.Exit(1)
	}

	return mapping
}
