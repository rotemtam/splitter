package splitter

import (
	"fmt"
	"github.com/alecthomas/kong"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"os"
	"path/filepath"
)

type (
	input struct {
		Source   string `arg:"" type:"existingfile" help:"Source HCL file to split."`
		DstDir   string `arg:"" type:"existingdir" help:"Destination directory to write the split files."`
		Strategy string `help:"Splitting strategy" enum:"schema,block" default:"schema"`
	}
	splitter func(*hcl.File) (map[string][]*hclsyntax.Block, error)
)

// Run split and return the exit code.
func Run() int {
	var cli input
	kong.Parse(&cli)
	if err := split(cli); err != nil {
		fmt.Printf("Error splitting HCL: %s\n", err)
		return 1
	}
	return 0
}

func (i input) splitter() splitter {
	switch i.Strategy {
	case "schema":
		return splitSchema
	default:
		return nil
	}
}

func split(i input) error {
	parse := hclparse.NewParser()
	file, diags := parse.ParseHCLFile(i.Source)
	if diags != nil && diags.HasErrors() {
		return diags
	}
	splitFn := i.splitter()
	if splitFn == nil {
		return fmt.Errorf("unknown splitting strategy %s", i.Strategy)
	}
	schemaBlocks, err := splitSchema(file)
	if err != nil {
		return err
	}
	for f, blocks := range schemaBlocks {
		outputPath := filepath.Join(i.DstDir, f)
		if err := writeFile(blocks, file, outputPath); err != nil {
			return err
		}
	}
	return nil
}

func splitSchema(file *hcl.File) (map[string][]*hclsyntax.Block, error) {
	schemaBlocks := make(map[string][]*hclsyntax.Block)
	noSchema := []*hclsyntax.Block{}
	body := file.Body.(*hclsyntax.Body)
	var schemas []*hclsyntax.Block
	// Detect all schemas.
	for _, block := range body.Blocks {
		if block.Type == "schema" {
			schemas = append(schemas, block)
			schemaBlocks[block.Labels[0]] = []*hclsyntax.Block{block}
		}
	}
	// Arrange all blocks by schema, placing those without a schema in a separate slice.
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
	return output, nil
}

func writeFile(blocks []*hclsyntax.Block, file *hcl.File, outputPath string) error {
	f := hclwrite.NewEmptyFile()
	rootBody := f.Body()

	for _, block := range blocks {
		hclBlock := hclwrite.NewBlock(block.Type, block.Labels)
		rootBody.AppendBlock(hclBlock)

		// Manually reconstruct the block body
		input := file.Bytes
		for attrName, attr := range block.Body.Attributes {
			exprTokens := attr.Expr.Range().SliceBytes(input)
			hclBlock.Body().SetAttributeRaw(attrName, hclwrite.TokensForTraversal(hcl.Traversal{
				hcl.TraverseRoot{Name: string(exprTokens)},
			}))
		}
		// Manually reconstruct nested blocks
		for _, nestedBlock := range block.Body.Blocks {
			nestedHCLBlock := hclwrite.NewBlock(nestedBlock.Type, nestedBlock.Labels)
			hclBlock.Body().AppendBlock(nestedHCLBlock)
			for attrName, attr := range nestedBlock.Body.Attributes {
				exprTokens := attr.Expr.Range().SliceBytes(input)
				nestedHCLBlock.Body().SetAttributeRaw(attrName, hclwrite.TokensForTraversal(hcl.Traversal{
					hcl.TraverseRoot{Name: string(exprTokens)},
				}))
			}
		}
	}
	if err := os.WriteFile(outputPath, f.Bytes(), 0644); err != nil {
		return err
	}
	return nil
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
