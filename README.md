# splt

A small command tool to split an Atlas HCL schema file into multiple files.

Supported strategies:
- `schema`: split by schema: each schema is written to a separate file.
- `block`: split by block: each block type is written to a separate file.

## Quickstart

Install:
```bash
go install github.com/rotemtam/splt/cmd/splt
```

Split a file:
```bash
# split by schema
splt schema.hcl out/

# split by block
splt schema.hcl out/ --strategy=block
````
  



## Usage

```
Usage: splt <source> <dst-dir> [flags]

Arguments:
  <source>     Source HCL file to split.
  <dst-dir>    Destination directory to write the split files.

Flags:
  -h, --help                 Show context-sensitive help.
      --strategy="schema"    Splitting strategy options:schema,block default:schema

```
