package splitter

import (
	"fmt"
	"github.com/alecthomas/kong"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"io"
	"os"
	"path/filepath"
	"sort"
)

type (
	input struct {
		Input    string `short:"i" help:"Input HCL file to split." type:"existingfile" optional:""`
		Output   string `short:"o" placeholder:"./path/to/dir" required:"" help:"Destination directory to write the split files." type:"existingdir"`
		Strategy string `help:"Splitting strategy options:schema,block" enum:"schema,block,resource" default:"schema"`
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
		outputPath := filepath.Join(i.Output, fileName)
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
		output["schema_"+name+".hcl"] = block
	}
	if len(noSchema) > 0 {
		output["main.hcl"] = noSchema
	}
	return output
}

func splitBlock(file *hcl.File) map[string][]*hclsyntax.Block {
	body := file.Body.(*hclsyntax.Body)
	output := make(map[string][]*hclsyntax.Block)
	for _, block := range body.Blocks {
		fname := block.Type + ".hcl"
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

	schemaBlocks := make(map[string]*hclsyntax.Block)
	output := make(map[string][]*hclsyntax.Block)

	for _, block := range body.Blocks {
		if block.Type == "schema" {
			schemaName := block.Labels[0]
			schemaBlocks[schemaName] = block
			schemaPath := fmt.Sprintf("schema_%s/schema.hcl", schemaName)
			output[schemaPath] = []*hclsyntax.Block{block}
		}
	}
	for _, block := range body.Blocks {
		if block.Type == "schema" {
			continue
		}
		schemaName, ok := detectSchema(block.Body)
		if !ok {
			continue
		}
		blockType := block.Type + "s" // pluralize
		fileName := fmt.Sprintf("schema_%s/%s/%s.hcl", schemaName, blockType, block.Labels[0])
		output[fileName] = []*hclsyntax.Block{block}
	}
	return output
}
