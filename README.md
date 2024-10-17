# splt

> Note: This experimental tool is not officially supported by Ariga or Atlas and is provided as-is.

A small command tool to split an Atlas HCL schema file into multiple files.

Accepts input schema via a file or stdin and writes the split files to a directory.

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
atlas schema inspect --url '<db url>' | splt --output=./out --strategy=schema 
```

Verify output is correct:
```bash
atlas schema diff --dev-url docker://postgres/16/dev --from <db url> --to file://out/
````

Output:
```
Schemas are synced, no changes to be made.
```

## Help

```
Usage: splt --output=./path/to/dir [flags]

Flags:
  -h, --help                    Show context-sensitive help.
  -i, --input=STRING            Input HCL file to split.
  -o, --output=./path/to/dir    Destination directory to write the split files.
      --strategy="schema"       Splitting strategy options:schema,block
```
