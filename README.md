# FASTA Sanitiser

## About

This program provides functionality to encode FASTA files with bioinformatics-safe headers, which it can then reliably
map back to the original headers produced by downstream analyses using the "decode" feature. This tool is aimed at
developers of community platforms that have to handle highly diverse user inputs and integrate 3rd party tools.

### Raw speed

Performance is a key feature. A 5MB FASTA file can be encoded in less than 1s, and decoded in <0.1s. A 1.3MB single line
JSON file will also decode in a similar time.

### CSV-safe mode

NB. Decoding may break some output formats (especially bespoke bioinformatics-style formats). However, to prevent tabs
or commas in FASTA headers from breaking CSV files, if the software detects that the file ends in `.csv` or `.tsv` it
will enclose the original identifier in double-quotes. This mode can be manually activated with the `-csv` option (see
below).

## Building the Program

To build the program, follow these steps:

1. Ensure you have Go installed on your system (version 1.22.1 or later recommended).
2. Clone this repository:

```
git clone https://github.com/CorinYeatsCGPS/sanitise-fasta.git
cd sanitise-fasta
```

3. Build the program:

```
go build
```

## Running the Program

The sanitiser program has two modes: encode and decode. Flags should be specified before the mode and input file. If no
arguments are provided, the program will display the help text.

### Arguments

1. [options]: Optional flags (see Options section below)
2. {mode}: Specifies the operation mode. Must be either "encode" or "decode".
3. {input}: Specifies the input file path. Use '-' to read from STDIN. "encode" mode only accepts FASTAs, while decode
   accepts any text file.

### Options

- `-store`: Specifies the store file path. If not provided, uses a default location.
- `-trim`: (Encode mode only) Specifies the number of characters to keep from the SHA1 checksum (max 40). Default is 40.
- `-csv`: (Decode mode only) Ensures original identifiers are quoted when written out to ensure CSV/TSV files don't
  break.

### Example Usage

1. Encode a FASTA file and specify the map data location:

```
./sanitiser -store id_store encode sequences.fasta > encoded_sequences.fasta
```

2. Encode a FASTA file with trimmed checksums:

```
./sanitiser -trim 20 encode sequences.fasta > encoded_sequences.fasta
```

3. Decode a file using a specified map file:

```
./sanitiser -store id_store decode encoded_data.txt > decoded_data.txt
```

4. Decode a CSV file safely:

```
./sanitiser -csv decode encoded_data.csv > decoded_data.csv
```

5.Encode from STDIN:

```
cat sequences.fasta | ./sanitiser encode - > encoded_sequences.fasta
```

### Warning

It is not possible to run the encoding and decoding concurrently, and so it will fail if you try to put them in the same command.

```
# Don't do this!
cat sequences.fasta | ./sanitiser encode - | my_program.py | ./sanitiser -csv decode - > results.csv
```

## About encoding

The encoding process transforms FASTA headers into a standardized format while preserving the ability to map back to the original headers. Here's how it works:

1. Input Processing:
    - The program reads the input FASTA file line by line.
    - Blank lines and lines starting with '#' or ';' are ignored.
    - The first non-ignored line must start with '>' to be considered a valid FASTA file.

2. Header Transformation:
    - For each sequence, the original header (everything after the '>' character) is stored in the mapping database.
    - A new header is generated using the format: >{index}_PW_{trimmed_hash}
        - {index}: A 1-based counter for the sequences in the file.
        - `_PW_`: A constant separator.
        - {trimmed_hash}: A SHA1 hash of the sequence, trimmed to the specified length (default 40 characters).

3. Output:
    - The new header is written, followed by the unchanged sequence data.
    - The process repeats for each sequence in the input file.

4. Mapping Storage:
    - The program stores a mapping between the new header and the original header in a persistent database.
    - This mapping is used later during the decoding process to restore the original headers.

By using this encoding method, the program ensures that all headers are in a consistent, predictable format, which can prevent issues with downstream tools that may be sensitive to certain characters or formats in FASTA headers.

## Author

This software was created by Corin Yeats for
the [Centre for Genomic Pathogen Surveillance](https://www.pathogensurveillance.net/).

## Project Structure

- sanitiser.go: Contains the main program logic and command-line interface.
- mapping_store.go: Handles the storage and retrieval of mappings.
- README.md: This file, containing instructions and information about the project.

## License

See [the License file](LICENSE).
