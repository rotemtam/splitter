# splitter

## Quickstart

Install:
```bash
go install github.com/rotemtam/splitter/cmd/splitter
```

Split a file:
```bash
# split by schema
splitter schema.hcl out/

# split by block
splitter schema.hcl out/ --strategy=block
````
  



## Usage

```
Usage: splitter <source> <dst-dir> [flags]

Arguments:
  <source>     Source HCL file to split.
  <dst-dir>    Destination directory to write the split files.

Flags:
  -h, --help                 Show context-sensitive help.
      --strategy="schema"    Splitting strategy options:schema,block default:schema

```
