# splt

> Note: This experimental tool is not officially supported by Ariga or Atlas and is provided as-is.

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
```
Verify output is correct:
```bash
atlas schema diff --dev-url docker://postgres/16/dev --from file://schema.hcl --to file://out/
````

Output:
```
Schemas are synced, no changes to be made.
```

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
