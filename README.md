# FASTA Sanitiser

This program provides functionality to encode and decode FASTA files, replacing sequence headers with unique identifiers
based on their content. The length of the identifiers can be controlled as well. The purpose of this software is to
replace FASTA headers with ones that are "guaranteed" to work reliably with downstream software. The "decode"
functionality can then be used to replace the encoded contig names with the correct contig names in output files.

Performance is a key feature. A 5MB FASTA file can be encoded in less than 1s, and decoded in <0.1s. A 1.3MB single line
JSON file will also decode in a similar time.

NB. This may break some output formats. Commas in headers in particular may break CSV files (and tabs may break TSV
files).

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

The sanitiser program has two modes: encode and decode. If no arguments are provided, the program will display the help
text.

### Encode Mode

Encode mode reads a FASTA format file, replaces the header of each sequence with a new identifier based on the index and
SHA1 checksum, and writes the output to STDOUT and creates a datastore of the mapped identifiers.

```
./sanitiser encode input.fasta > output.fasta
```

To read from STDIN, use '-' as the input file and `-store` to write the mapping data to a specified location:

```
cat input.fasta | ./sanitiser encode - -store map.data > output.fasta
```

### Decode Mode

Decode mode reads an arbitrary text file and uses the stored mappings to replace the new identifiers with the original
ones, writing the output to STDOUT.

```
./sanitiser decode input.txt > output.txt
```

To read from STDIN, use '-' as the input file:

```
cat input.txt | ./sanitiser decode - > output.txt
```

## Arguments

- {mode}: Specifies the operation mode. Must be either "encode" or "decode".
- {input}: Specifies the input file path. Use '-' to read from STDIN.

## Options

- `-store`: Specifies the store file path. If not provided, uses a default location.
- `-trim`: (Encode mode only) Specifies the number of characters to keep from the SHA1 checksum (max 40). Default is 40.

## Example Usage

1. Encode a FASTA file and specify the map data location:

```
./sanitiser encode sequences.fasta -store id_store > encoded_sequences.fasta
```

2. Encode a FASTA file with trimmed checksums:

```
./sanitiser encode sequences.fasta -trim 20 > encoded_sequences.fasta
```

3. Decode a file using a specified map file:

```
./sanitiser decode encoded_data.txt -store id_store > decoded_data.txt
```

4. Encode from STDIN:

```
cat sequences.fasta | ./sanitiser encode - > encoded_sequences.fasta
```

This program efficiently handles large FASTA files and provides a way to anonymize sequence identifiers while
maintaining the ability to map them back to their original values.

## Project Structure

- sanitiser.go: Contains the main program logic and command-line interface.
- mapping_store.go: Handles the storage and retrieval of mappings.
- README.md: This file, containing instructions and information about the project.

## License

See [the License file](License.md)
