package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shopspring/decimal"

	"laguna-escondida/backend/internal/domain/dto"
)

// maxCatalogImportFileBytes matches the import endpoints' body cap.
const maxCatalogImportFileBytes = 2 << 20

const catalogImportDocumentHelp = "the catalog import document (defaults, existing, ingredients, sellables, plates, opening_stock); entries link by ref, see docs/api/catalog-import.md"

const catalogImportFilePathHelp = "path to a JSON file holding the document, on the machine running this MCP server; accepted only over stdio (Claude Desktop), at most 2 MiB"

type validateCatalogImportInput struct {
	Document *dto.CatalogImportDocument `json:"document,omitempty" jsonschema:"catalog import document; give this or file_path"`
	FilePath string                     `json:"file_path,omitempty" jsonschema:"JSON file with the document; give this or document"`
}

type applyCatalogImportInput struct {
	Document       *dto.CatalogImportDocument `json:"document,omitempty" jsonschema:"catalog import document; give this or file_path"`
	FilePath       string                     `json:"file_path,omitempty" jsonschema:"JSON file with the document; give this or document"`
	ExpectedDigest string                     `json:"expected_digest" jsonschema:"the digest validate_catalog_import returned for this exact document"`
}

var errFilePathNeedsStdio = errors.New("file_path is only accepted when this MCP server runs on stdio (Claude Desktop on your own machine); pass the document inline instead")

// importToolSchema reflects the tool input, with decimals as a JSON number or numeric string,
// which is how the backend decodes them.
func importToolSchema[T any](documentHelp, filePathHelp string) *jsonschema.Schema {
	schema, err := jsonschema.For[T](&jsonschema.ForOptions{TypeSchemas: map[reflect.Type]*jsonschema.Schema{
		reflect.TypeFor[decimal.Decimal](): {Types: []string{"number", "string"}},
	}})
	if err != nil {
		panic(fmt.Sprintf("catalog import tool schema: %v", err))
	}
	schema.Properties["document"].Description = documentHelp
	schema.Properties["file_path"].Description = filePathHelp
	return schema
}

func registerCatalogImportTools(s *mcp.Server, c *Client, localFiles bool) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "validate_catalog_import",
		Description: "Dry-run a whole catalog import (cloud only, admin). Writes nothing. Returns valid, every problem with a JSON-pointer path and a stable code, " +
			"a summary to show the operator, and the digest that apply_catalog_import requires. Follow the onboard-catalog playbook.",
		InputSchema: importToolSchema[validateCatalogImportInput](catalogImportDocumentHelp, catalogImportFilePathHelp),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in validateCatalogImportInput) (*mcp.CallToolResult, any, error) {
		doc, err := catalogImportDocument(in.Document, in.FilePath, localFiles)
		if err != nil {
			return toolError(err)
		}
		return forwardCatalogImport(ctx, c, "/api/catalog/import/validate", doc)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "apply_catalog_import",
		Description: "Apply a validated catalog import in one transaction (cloud only, admin): it creates every product, recipe line, option slot, preparation area and opening count, or nothing. " +
			"Call it only after the operator explicitly approved the validation summary, with the same document and its expected_digest.",
		InputSchema: importToolSchema[applyCatalogImportInput](catalogImportDocumentHelp, catalogImportFilePathHelp),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in applyCatalogImportInput) (*mcp.CallToolResult, any, error) {
		if in.ExpectedDigest == "" {
			return toolError(errors.New("expected_digest is required: validate the document first and apply it with the digest the validation returned"))
		}
		doc, err := catalogImportDocument(in.Document, in.FilePath, localFiles)
		if err != nil {
			return toolError(err)
		}
		return forwardCatalogImport(ctx, c, "/api/catalog/import", dto.ApplyCatalogImportRequest{CatalogImportDocument: *doc, ExpectedDigest: in.ExpectedDigest})
	})
}

func catalogImportDocument(inline *dto.CatalogImportDocument, filePath string, localFiles bool) (*dto.CatalogImportDocument, error) {
	switch {
	case inline != nil && filePath != "":
		return nil, errors.New("give either document or file_path, not both")
	case inline != nil:
		return inline, nil
	case filePath == "":
		return nil, errors.New("give the document inline or as file_path")
	case !localFiles:
		return nil, errFilePathNeedsStdio
	}
	return readCatalogImportFile(filePath)
}

// readCatalogImportFile decodes the file the way the endpoint decodes a body: unknown fields
// refused, so a field the backend would reject is not silently dropped on the way.
func readCatalogImportFile(path string) (*dto.CatalogImportDocument, error) {
	f, err := os.Open(path) //nolint:gosec // reached only with WithLocalFiles (stdio, the operator's own machine)
	if err != nil {
		return nil, fmt.Errorf("open catalog import file: %w", err)
	}
	defer func() { _ = f.Close() }()
	raw, err := io.ReadAll(io.LimitReader(f, maxCatalogImportFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read catalog import file: %w", err)
	}
	if len(raw) > maxCatalogImportFileBytes {
		return nil, fmt.Errorf("catalog import file is larger than %d bytes", maxCatalogImportFileBytes)
	}
	var doc dto.CatalogImportDocument
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("catalog import file is not a valid document: %w", err)
	}
	return &doc, nil
}

// forwardCatalogImport returns the backend's body unchanged. A 409 or 422 still carries the
// problems the model needs, so it is returned as the tool's text, flagged as an error.
func forwardCatalogImport(ctx context.Context, c *Client, path string, body any) (*mcp.CallToolResult, any, error) {
	resp, err := c.Post(ctx, path, body)
	if err == nil {
		return ok(resp)
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) && json.Valid(apiErr.Body) && !isErrorOnlyBody(apiErr.Body) {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: string(apiErr.Body)}}}, nil, nil
	}
	return toolError(err)
}

func isErrorOnlyBody(body []byte) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil {
		return false
	}
	_, hasProblems := fields["problems"]
	return !hasProblems
}
