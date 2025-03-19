# FASTA Sanitiser

This program provides functionality to encode and decode FASTA files, replacing sequence headers with unique identifiers
based on their content. The length of the identifiers can be controlled as well.

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
go build -o sanitiser
```

## Running the Program

The sanitiser program has two modes: encode and decode. If no arguments are provided, the program will display the help
text.

### Encode Mode

Encode mode reads a FASTA format file, replaces the header of each sequence with a new identifier based on the index and
SHA1 checksum, and writes the output to STDOUT along with a mapping file.

```
./sanitiser -mode encode -input input.fasta > output.fasta
```

To read from STDIN, use '-' as the input file:

```
cat input.fasta | ./sanitiser -mode encode -input - -mapping mapfile > output.fasta
```

### Decode Mode

Decode mode reads an arbitrary text file and uses the mapping file to replace the new identifiers with the original
ones, writing the output to STDOUT.

```
./sanitiser -mode decode -input input.txt > output.txt
```

To read from STDIN, use '-' as the input file:

```
cat input.txt | ./sanitiser -mode decode -input - > output.txt
```

## Options

- `-mode`: Specifies the operation mode. Must be either "encode" or "decode".
- `-input`: Specifies the input file path. Use '-' to read from STDIN. If not provided, the program reads from STDIN.
- `-mapping`: Specifies the mapping file path. If not provided, uses a default location.
- `-trim`: (Encode mode only) Specifies the number of characters to keep from the SHA1 checksum (max 40). Default is 40.

## Example Usage

1. Encode a FASTA file:

```
./sanitiser -mode encode -input sequences.fasta -mapping id_mapping.txt > encoded_sequences.fasta
```

2. Encode a FASTA file with trimmed checksums:

```
./sanitiser -mode encode -input sequences.fasta -mapping id_mapping.txt -trim 20 > encoded_sequences.fasta
```

3. Decode a file using the mapping:

```
 ./sanitiser -mode decode -input encoded_data.txt -mapping id_mapping.txt > decoded_data.txt
 ```

4. Encode from STDIN:

```
5. cat sequences.fasta | ./sanitiser -mode encode -input - -mapping id_mapping.txt > encoded_sequences.fasta
```

This program efficiently handles large FASTA files and provides a way to anonymize sequence identifiers while
maintaining the ability to map them back to their original values.

## Project Structure

- sanitiser.go: Contains the main program logic and command-line interface.
- mapping_store.go: Handles the storage and retrieval of mappings.
- README.md: This file, containing instructions and information about the project.

## License

See [the License file](License.md)
