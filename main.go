package splitter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

func main() {
	inputPath := filepath.Join("testdata", "simple", "schema.hcl")

	input, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Printf("Error reading input file: %s\n", err)
		return
	}

	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(input, inputPath)
	if diags.HasErrors() {
		fmt.Printf("Error parsing HCL: %s\n", diags)
		return
	}

	// Group blocks by schema
	schemaBlocks := make(map[string][]*hclsyntax.Block)
	var schemas []*hclsyntax.Block

	body := file.Body.(*hclsyntax.Body)
	for _, block := range body.Blocks {
		if block.Type == "schema" {
			schemas = append(schemas, block)
			schemaBlocks[block.Labels[0]] = []*hclsyntax.Block{block}
		} else if block.Type == "table" {
			for _, attr := range block.Body.Attributes {
				if attr.Name == "schema" {
					if expr, ok := attr.Expr.(*hclsyntax.ScopeTraversalExpr); ok {
						if len(expr.Traversal) == 2 && expr.Traversal[0].(hcl.TraverseRoot).Name == "schema" {
							schemaName := expr.Traversal[1].(hcl.TraverseAttr).Name
							schemaBlocks[schemaName] = append(schemaBlocks[schemaName], block)
						}
					}
				}
			}
		}
	}

	// Write separate files for each schema
	for schemaName, blocks := range schemaBlocks {
		outputPath := filepath.Join("testdata", "simple", fmt.Sprintf("%s.hcl", schemaName))

		f := hclwrite.NewEmptyFile()
		rootBody := f.Body()

		for _, block := range blocks {
			hclBlock := hclwrite.NewBlock(block.Type, block.Labels)
			rootBody.AppendBlock(hclBlock)

			// Manually reconstruct the block body
			for attrName, attr := range block.Body.Attributes {
				exprTokens := attr.Expr.Range().SliceBytes(input)
				hclBlock.Body().SetAttributeRaw(attrName, hclwrite.TokensForTraversal(hcl.Traversal{
					hcl.TraverseRoot{Name: string(exprTokens)},
				}))
			}
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

		err = os.WriteFile(outputPath, f.Bytes(), 0644)
		if err != nil {
			fmt.Printf("Error writing output file %s: %s\n", outputPath, err)
			continue
		}

		fmt.Printf("Created file for schema '%s': %s\n", schemaName, outputPath)
	}
}
