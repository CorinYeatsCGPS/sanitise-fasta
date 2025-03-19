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
	"runtime"
	"strings"
)

const (
	idFormat      = "%d___%s"
	idRegexFormat = `\d+___[a-f0-9]+`
)

func main() {
	storeLocation := flag.String("store", "", "Location to store mapping data (optional, uses current directory if not provided)")
	trimLength := flag.Int("trim", 40, "Number of characters to keep from the SHA1 checksum (optional, uses 40 if not provided). Maximum is 40.")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [encode|decode] <input_file> [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample usage:\n")
		fmt.Fprintf(os.Stderr, "  Encode: %s encode input.fasta > output.fasta\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  Decode: %s decode input.txt > output.txt\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  Use '-' as input_file to read from STDIN\n")
	}

	flag.Parse()

	if flag.NArg() != 2 || (flag.Arg(0) != "encode" && flag.Arg(0) != "decode") {
		flag.Usage()
		os.Exit(1)
	}

	mode := flag.Arg(0)
	inputFile := flag.Arg(1)

	if *trimLength < 1 || *trimLength > 40 {
		fmt.Fprintf(os.Stderr, "Error: trim value must be between 1 and 40\n")
		os.Exit(1)
	}

	var input io.Reader
	if inputFile == "-" {
		input = os.Stdin
	} else {
		file, err := os.Open(inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening input file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()
		input = file
	}

	switch mode {
	case "encode":
		mappingStore, err := NewMappingStore(*storeLocation, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating mapping store: %v\n", err)
			os.Exit(1)
		}
		defer mappingStore.Close()
		encodeMode(input, mappingStore, *trimLength)

	case "decode":
		mappingStore, err := NewMappingStore(*storeLocation, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating mapping store: %v\n", err)
			os.Exit(1)
		}
		defer mappingStore.Close()
		decodeMode(input, mappingStore)
	}
}

func encodeMode(input io.Reader, mappingStore *MappingStore, trimLength int) {
	scanner := bufio.NewScanner(input)
	writer := bufio.NewWriter(os.Stdout)
	defer func() {
		if err := writer.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "Error flushing writer: %v\n", err)
		}
	}()

	var currentHeader, currentSequence string
	index := 0

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, ">") {
			if currentHeader != "" {
				if err := processSequence(currentHeader, currentSequence, index, mappingStore, writer, trimLength); err != nil {
					fmt.Fprintf(os.Stderr, "Error processing sequence: %v\n", err)
					os.Exit(1)
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
		if err := processSequence(currentHeader, currentSequence, index, mappingStore, writer, trimLength); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing sequence: %v\n", err)
			os.Exit(1)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}

	// Finalize the database (commit transaction, create index, analyze)
	if err := mappingStore.Finalise(); err != nil {
		fmt.Fprintf(os.Stderr, "Error finalizing database: %v\n", err)
		os.Exit(1)
	}

	if _, err := fmt.Fprintf(os.Stderr, "Encoding completed. Database optimized.\n"); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing completion message: %v\n", err)
	}
}

func processSequence(header, sequence string, index int, mappingStore *MappingStore, writer *bufio.Writer, trimLength int) error {
	hash := sha1.Sum([]byte(sequence))
	trimmedHash := hex.EncodeToString(hash[:])[:trimLength]
	newID := fmt.Sprintf(idFormat, index+1, trimmedHash)
	newHeader := fmt.Sprintf(">%s", newID)

	if err := mappingStore.StorePair(newID, header); err != nil {
		return fmt.Errorf("error storing mapping: %v", err)
	}

	_, err := fmt.Fprintf(writer, "%s\n%s\n", newHeader, sequence)
	if err != nil {
		return fmt.Errorf("error writing sequence: %v", err)
	}

	return nil
}

func decodeMode(input io.Reader, mappingStore *MappingStore) {
	scanner := bufio.NewScanner(input)
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()

	// Increase the buffer size to handle larger lines
	const maxCapacity = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	// Regular expression to match the encoded IDs
	re := regexp.MustCompile(idRegexFormat)

	// Create a worker pool
	numWorkers := runtime.NumCPU()
	jobs := make(chan job, numWorkers)
	results := make(chan result, numWorkers)

	// Start worker goroutines
	for i := 0; i < numWorkers; i++ {
		go worker(jobs, results, re, mappingStore)
	}

	// Process lines and collect results
	go func() {
		lineNum := 0
		for scanner.Scan() {
			jobs <- job{lineNum: lineNum, line: scanner.Text()}
			lineNum++
		}
		close(jobs)
	}()

	// Collect and write results in order
	resultCache := make(map[int]string)
	nextLineToWrite := 0
	jobsProcessed := 0
	totalJobs := 0

	for res := range results {
		jobsProcessed++
		totalJobs = max(totalJobs, res.lineNum+1)
		resultCache[res.lineNum] = res.line

		for {
			if line, ok := resultCache[nextLineToWrite]; ok {
				if _, err := fmt.Fprintln(writer, line); err != nil {
					fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
					os.Exit(1)
				}
				delete(resultCache, nextLineToWrite)
				nextLineToWrite++
			} else {
				break
			}
		}

		if jobsProcessed == totalJobs {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}

	close(results)
}

type job struct {
	lineNum int
	line    string
}

type result struct {
	lineNum int
	line    string
}

func worker(jobs <-chan job, results chan<- result, re *regexp.Regexp, mappingStore *MappingStore) {
	for j := range jobs {
		// Find all matches in the line
		matches := re.FindAllString(j.line, -1)

		if len(matches) > 0 {
			// Create a map of replacements
			replacements := make(map[string]string)
			for _, match := range matches {
				originalID, err := mappingStore.LookupOriginalID(match)
				if err == nil {
					replacements[match] = originalID
				} else {
					fmt.Fprintf(os.Stderr, "Warning: Could not decode ID %s: %v\n", match, err)
				}
			}

			// Perform a single pass replacement
			for oldID, newID := range replacements {
				j.line = strings.ReplaceAll(j.line, oldID, newID)
			}
		}

		results <- result{lineNum: j.lineNum, line: j.line}
	}
}
