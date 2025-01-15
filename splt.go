package splitter

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

type (
	input struct {
		Input     string `short:"i" help:"Input HCL file to split." type:"existingfile" optional:""`
		Output    string `short:"o" placeholder:"./path/to/dir" required:"" help:"Destination directory to write the split files." type:"existingdir"`
		Strategy  string `help:"Splitting strategy options:schema,block,resource" enum:"schema,block,resource" default:"schema"`
		Extension string `help:"Output file extension" default:"hcl"`
	}
	strategy func(*hcl.File) map[string][]*hclsyntax.Block
)

// Run split and return the exit code.
func Run() int {
	var cli input
	kong.Parse(&cli)
	if err := split(cli); err != nil {
		fmt.Fprintf(os.Stderr, "splt: %s\n", err)
		return 1
	}
	return 0
}

func (i input) strategy() strategy {
	switch i.Strategy {
	case "schema":
		return splitSchema
	case "block":
		return splitBlock
	case "resource":
		return splitResource
	default:
		return nil
	}
}

// Modify the existing split function to create directories
func split(i input) error {
	parse := hclparse.NewParser()
	var (
		file  *hcl.File
		diags hcl.Diagnostics
	)
	if i.Input != "" {
		file, diags = parse.ParseHCLFile(i.Input)
	} else {
		stat, err := os.Stdin.Stat()
		if err != nil || (stat.Mode()&os.ModeCharDevice) != 0 {
			return fmt.Errorf("no input file provided")
		}
		all, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}
		if len(all) == 0 {
			return fmt.Errorf("no input provided, provide input via stdin or -i flag")
		}
		file, diags = parse.ParseHCL(all, "stdin.hcl")
	}
	if diags != nil && diags.HasErrors() {
		return diags
	}
	splitFn := i.strategy()
	if splitFn == nil {
		return fmt.Errorf("unknown splitting strategy %s", i.Strategy)
	}
	files := splitFn(file)
	for fileName := range files {
		dir := filepath.Dir(filepath.Join(i.Output, fileName))
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}
	for fileName, blocks := range files {
		outputPath := filepath.Join(i.Output, fmt.Sprintf("%s.%s", fileName, i.Extension))
		if err := writeFile(blocks, file, outputPath); err != nil {
			return err
		}
	}
	return nil
}

func splitSchema(file *hcl.File) map[string][]*hclsyntax.Block {
	schemaBlocks := make(map[string][]*hclsyntax.Block)
	noSchema := []*hclsyntax.Block{}
	body := file.Body.(*hclsyntax.Body)
	var schemas []*hclsyntax.Block
	for _, block := range body.Blocks {
		if block.Type == "schema" {
			schemas = append(schemas, block)
			schemaBlocks[block.Labels[0]] = []*hclsyntax.Block{block}
		}
	}
	for _, block := range body.Blocks {
		if block.Type == "schema" {
			continue
		}
		name, ok := detectSchema(block.Body)
		if !ok {
			noSchema = append(noSchema, block)
			continue
		}
		schemaBlocks[name] = append(schemaBlocks[name], block)
	}
	output := make(map[string][]*hclsyntax.Block, len(schemas)+1)
	for name, block := range schemaBlocks {
		output[schemaFile(name)] = block
	}
	if len(noSchema) > 0 {
		output["main"] = noSchema
	}
	return output
}

func splitBlock(file *hcl.File) map[string][]*hclsyntax.Block {
	body := file.Body.(*hclsyntax.Body)
	output := make(map[string][]*hclsyntax.Block)
	for _, block := range body.Blocks {
		fname := block.Type
		if _, ok := output[fname]; !ok {
			output[fname] = []*hclsyntax.Block{}
		}
		output[fname] = append(output[fname], block)
	}
	return output
}

func detectSchema(body *hclsyntax.Body) (string, bool) {
	for _, attr := range body.Attributes {
		if attr.Name == "schema" {
			if expr, ok := attr.Expr.(*hclsyntax.ScopeTraversalExpr); ok {
				if len(expr.Traversal) == 2 && expr.Traversal[0].(hcl.TraverseRoot).Name == "schema" {
					name := expr.Traversal[1].(hcl.TraverseAttr).Name
					return name, true
				}
			}
		}
	}
	return "", false
}

func writeFile(blocks []*hclsyntax.Block, file *hcl.File, outputPath string) error {
	f := hclwrite.NewEmptyFile()
	rootBody := f.Body()
	src := file.Bytes
	var writeBlock func(*hclwrite.Body, *hclsyntax.Block)
	writeBlock = func(body *hclwrite.Body, block *hclsyntax.Block) {
		hclBlock := body.AppendNewBlock(block.Type, block.Labels)
		blockBody := hclBlock.Body()
		var attrs []*hclsyntax.Attribute
		for _, attr := range block.Body.Attributes {
			attrs = append(attrs, attr)
		}
		sort.Slice(attrs, func(i, j int) bool {
			return attrs[i].NameRange.Start.Byte < attrs[j].NameRange.Start.Byte
		})
		for _, attr := range attrs {
			exprTokens := attr.Expr.Range().SliceBytes(src)
			blockBody.SetAttributeRaw(attr.Name, hclwrite.Tokens{
				{Type: hclsyntax.TokenIdent, Bytes: exprTokens},
			})
		}
		for _, nestedBlock := range block.Body.Blocks {
			writeBlock(blockBody, nestedBlock)
		}
	}
	for _, block := range blocks {
		writeBlock(rootBody, block)
	}
	return os.WriteFile(outputPath, f.Bytes(), 0644)
}

func splitResource(file *hcl.File) map[string][]*hclsyntax.Block {
	body := file.Body.(*hclsyntax.Body)
	var (
		schemaBlocks = make(map[string]*hclsyntax.Block)
		tableBlocks  = make(map[string]*hclsyntax.Block)
		output       = make(map[string][]*hclsyntax.Block)
		triggers     []*hclsyntax.Block
		noSchema     []*hclsyntax.Block
	)
	for _, block := range body.Blocks {
		if block.Type == "schema" {
			schemaName := block.Labels[0]
			schemaBlocks[schemaName] = block
			schemaPath := filepath.Join(schemaFile(schemaName), "schema")
			output[schemaPath] = []*hclsyntax.Block{block}
		}
		if block.Type == "table" {
			tableBlocks[blockAddr(block)] = block
		}
	}
	for _, block := range body.Blocks {
		switch block.Type {
		case "schema":
			continue
		case "trigger":
			triggers = append(triggers, block)
		default:
			schemaName, ok := detectSchema(block.Body)
			if !ok {
				noSchema = append(noSchema, block)
				continue
			}
			blockType := block.Type + "s"
			tn := block.Labels[len(block.Labels)-1] // Resource blocks may be qualified with schema name.
			fileName := filepath.Join(schemaFile(schemaName), blockType, tn)
			output[fileName] = []*hclsyntax.Block{block}
		}
	}
	if len(triggers) > 0 {
		for _, trigger := range triggers {
			addr, ok := onAddr(file, trigger)
			if !ok {
				continue
			}
			tableBlock, ok := tableBlocks[addr]
			if !ok {
				continue
			}
			schemaName, ok := detectSchema(tableBlock.Body)
			if !ok {
				continue
			}
			fields := strings.Split(addr, ".")
			tableName := fields[len(fields)-1]
			fileName := filepath.Join(schemaFile(schemaName), "tables", tableName)
			output[fileName] = append(output[fileName], trigger)
		}
	}
	if len(noSchema) > 0 {
		output["main"] = noSchema
	}
	return output
}

func blockAddr(b *hclsyntax.Block) string {
	return fmt.Sprintf("%s.%s", b.Type, strings.Join(b.Labels, "."))
}

func onAddr(file *hcl.File, b *hclsyntax.Block) (string, bool) {
	on, ok := b.Body.Attributes["on"]
	if !ok {
		return "", false
	}
	rng := on.Expr.Range()
	return string(file.Bytes[rng.Start.Byte:rng.End.Byte]), true
}

func schemaFile(s string) string {
	return "schema_" + s
}
