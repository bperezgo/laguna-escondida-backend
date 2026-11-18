package mcpserver

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// maxInlineXMLBytes caps how large an extracted XML file may be before its text
// is inlined in the response. Colombian DIAN invoice XML is tens to a few
// hundred KB; anything larger is likely not an invoice and is left on disk.
const maxInlineXMLBytes = 2_000_000

type extractArchiveInput struct {
	ArchivePath string `json:"archive_path" jsonschema:"absolute path on the MCP server host to a .zip archive to extract (e.g. a supplier invoice bundle)"`
	DestDir     string `json:"dest_dir,omitempty" jsonschema:"optional destination directory; defaults to a folder beside the archive named after it"`
}

type extractedFile struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type extractArchiveOutput struct {
	DestDir     string            `json:"dest_dir"`
	Extracted   []extractedFile   `json:"extracted"`
	PDFs        []string          `json:"pdfs"`
	XMLFiles    []string          `json:"xml_files"`
	XMLContents map[string]string `json:"xml_contents,omitempty"`
	Notes       []string          `json:"notes,omitempty"`
}

// registerFileTools registers host-local file utilities. Unlike the other tools,
// these operate on the MCP server host's filesystem (like the *_document uploads
// do), not the backend API — they exist so a local client such as Claude Desktop,
// which has no shell, can still unpack an invoice bundle before ingesting it.
func registerFileTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "extract_archive",
		Description: "Extract a .zip archive on the MCP server host (e.g. a Colombian DIAN invoice bundle) and return the extracted file paths. Inlines the text of any XML files so the invoice data can be read directly; PDFs are left on disk for reading/attaching.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in extractArchiveInput) (*mcp.CallToolResult, any, error) {
		out, err := extractArchive(in.ArchivePath, in.DestDir)
		if err != nil {
			return toolError(err)
		}
		body, err := json.Marshal(out)
		if err != nil {
			return toolError(fmt.Errorf("marshal extract result: %w", err))
		}
		return ok(body)
	})
}

func extractArchive(archivePath, destDir string) (*extractArchiveOutput, error) {
	if strings.ToLower(filepath.Ext(archivePath)) != ".zip" {
		return nil, fmt.Errorf("extract_archive only supports .zip files, got %q", filepath.Base(archivePath))
	}

	if destDir == "" {
		base := strings.TrimSuffix(filepath.Base(archivePath), filepath.Ext(archivePath))
		destDir = filepath.Join(filepath.Dir(archivePath), base)
	}
	destRoot, err := filepath.Abs(destDir)
	if err != nil {
		return nil, fmt.Errorf("resolve dest dir: %w", err)
	}
	if mkErr := os.MkdirAll(destRoot, 0o750); mkErr != nil {
		return nil, fmt.Errorf("create dest dir: %w", mkErr)
	}

	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open archive %q: %w", archivePath, err)
	}
	defer func() { _ = r.Close() }()

	out := &extractArchiveOutput{DestDir: destRoot, XMLContents: map[string]string{}}
	for _, f := range r.File {
		// Guard against zip-slip: the resolved target must stay inside destRoot.
		target := filepath.Join(destRoot, f.Name) //nolint:gosec // validated below
		if target != destRoot && !strings.HasPrefix(target, destRoot+string(os.PathSeparator)) {
			return nil, fmt.Errorf("refusing path traversal in archive entry %q", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o750); err != nil {
				return nil, fmt.Errorf("create dir %q: %w", f.Name, err)
			}
			continue
		}
		if err := writeZipEntry(f, target); err != nil {
			return nil, err
		}
		info, _ := os.Stat(target)
		var size int64
		if info != nil {
			size = info.Size()
		}
		out.Extracted = append(out.Extracted, extractedFile{Path: target, Name: filepath.Base(target), Size: size})

		switch strings.ToLower(filepath.Ext(target)) {
		case ".pdf":
			out.PDFs = append(out.PDFs, target)
		case ".xml":
			out.XMLFiles = append(out.XMLFiles, target)
			if size <= maxInlineXMLBytes {
				if content, readErr := os.ReadFile(target); readErr == nil { //nolint:gosec // target is validated against zip-slip above
					out.XMLContents[target] = string(content)
				}
			} else {
				out.Notes = append(out.Notes, fmt.Sprintf("%s is %d bytes; too large to inline, read it from disk", filepath.Base(target), size))
			}
		}
	}
	return out, nil
}

func writeZipEntry(f *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return fmt.Errorf("create parent for %q: %w", f.Name, err)
	}
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("open entry %q: %w", f.Name, err)
	}
	defer func() { _ = rc.Close() }()

	dst, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) //nolint:gosec // target is validated against zip-slip by the caller
	if err != nil {
		return fmt.Errorf("create %q: %w", target, err)
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, rc); err != nil { //nolint:gosec // invoice archives are operator-provided, bounded input
		return fmt.Errorf("write %q: %w", target, err)
	}
	return nil
}
