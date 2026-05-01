package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
)

func (fs *FilesystemHandler) HandleFileTree(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	path, _ := request.RequireString("starting_path")
	if path == "." || path == "./" {
		cwd, _ := os.Getwd()
		path = cwd
	}

	depth := 3
	if d, err := request.RequireFloat("max_depth"); err == nil {
		depth = int(d)
	}

	followSymlinks := false
	if f, err := request.RequireBool("follow_symlinks"); err == nil {
		followSymlinks = f
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return errorResult(err), nil
	}

	// Build the tree and get totals
	tree, totalFiles, totalDirs, err := fs.buildFileTreeWithTotals(validPath, depth, 0, followSymlinks)
	if err != nil {
		return errorResult(err), nil
	}

	jsonData, _ := json.MarshalIndent(tree, "", "  ")
	resourceURI := pathToResourceURI(validPath)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("File tree for %s\nTotals (to max. depth %d): %d files, %d directories\n\n%s", validPath, depth, totalFiles, totalDirs, string(jsonData)),
			},
			mcp.EmbeddedResource{
				Type: "resource",
				Resource: mcp.TextResourceContents{
					URI:      resourceURI,
					MIMEType: "application/json",
					Text:     string(jsonData),
				},
			},
		},
	}, nil
}

// buildFileTree builds a tree representation of the filesystem starting at the given starting path
func (fs *FilesystemHandler) HandleDirectoryTree(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	path, _ := request.RequireString("starting_path")
	if path == "." || path == "./" {
		cwd, _ := os.Getwd()
		path = cwd
	}

	depth := 3
	if d, err := request.RequireFloat("max_depth"); err == nil {
		depth = int(d)
	}

	followSymlinks := false
	if f, err := request.RequireBool("follow_symlinks"); err == nil {
		followSymlinks = f
	}

	validPath, err := fs.validatePath(path)
	if err != nil {
		return errorResult(err), nil
	}

	// Build the directory tree and get totals
	tree, totalFiles, totalDirs, err := fs.buildDirectoryTreeWithTotals(validPath, depth, 0, followSymlinks)
	if err != nil {
		return errorResult(err), nil
	}

	jsonData, _ := json.MarshalIndent(tree, "", "  ")
	resourceURI := pathToResourceURI(validPath)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Directory tree for %s\nTotals (to max. depth %d): %d files, %d directories\n\n%s", validPath, depth, totalFiles, totalDirs, string(jsonData)),
			},
			mcp.EmbeddedResource{
				Type: "resource",
				Resource: mcp.TextResourceContents{
					URI:      resourceURI,
					MIMEType: "application/json",
					Text:     string(jsonData),
				},
			},
		},
	}, nil
}

// buildFileTreeWithTotals builds a full tree and tracks cumulative file/directory counts.
func (fs *FilesystemHandler) buildFileTreeWithTotals(path string, maxDepth int, currentDepth int, followSymlinks bool) (*FileNode, int, int, error) {
	validPath, err := fs.validatePath(path)
	if err != nil {
		return nil, 0, 0, err
	}

	info, err := os.Stat(validPath)
	if err != nil {
		return nil, 0, 0, err
	}

	node := &FileNode{
		Name:     filepath.Base(validPath),
		Path:     validPath,
		Modified: info.ModTime(),
	}

	if !info.IsDir() {
		node.Type = "file"
		node.Size = info.Size()
		return node, 1, 0, nil
	}

	node.Type = "directory"
	totalFiles := 0
	totalDirs := 1

	if currentDepth < maxDepth {
		entries, err := os.ReadDir(validPath)
		if err != nil {
			return node, totalFiles, totalDirs, nil
		}

		for _, entry := range entries {
			entryPath := filepath.Join(validPath, entry.Name())
			if entry.Type()&os.ModeSymlink != 0 && followSymlinks {
				linkDest, err := filepath.EvalSymlinks(entryPath)
				if err == nil && fs.isPathInAllowedDirs(linkDest) {
					entryPath = linkDest
				}
			}

			child, subFiles, subDirs, err := fs.buildFileTreeWithTotals(entryPath, maxDepth, currentDepth+1, followSymlinks)
			if err == nil {
				node.Children = append(node.Children, child)
				totalFiles += subFiles
				totalDirs += subDirs
			}
		}
	}

	return node, totalFiles, totalDirs, nil
}

// buildDirectoryTreeWithTotals builds a directory-only tree with file counts per directory and global totals.
func (fs *FilesystemHandler) buildDirectoryTreeWithTotals(path string, maxDepth int, currentDepth int, followSymlinks bool) (*DirectoryNode, int, int, error) {
	validPath, err := fs.validatePath(path)
	if err != nil {
		return nil, 0, 0, err
	}

	info, err := os.Stat(validPath)
	if err != nil {
		return nil, 0, 0, err
	}

	if !info.IsDir() {
		return nil, 0, 0, fmt.Errorf("not a directory")
	}

	node := &DirectoryNode{
		Name:     filepath.Base(validPath),
		Path:     validPath,
		Modified: info.ModTime(),
		Children: []*DirectoryNode{},
	}

	entries, err := os.ReadDir(validPath)
	if err != nil {
		return node, 0, 1, nil
	}

	localFiles := 0
	totalFiles := 0
	totalDirs := 1

	for _, entry := range entries {
		entryPath := filepath.Join(validPath, entry.Name())
		isDir := entry.IsDir()

		if entry.Type()&os.ModeSymlink != 0 && followSymlinks {
			res, err := filepath.EvalSymlinks(entryPath)
			if err == nil && fs.isPathInAllowedDirs(res) {
				if s, err := os.Stat(res); err == nil {
					isDir = s.IsDir()
					entryPath = res
				}
			}
		}

		if !isDir {
			localFiles++
			totalFiles++
		} else if currentDepth < maxDepth {
			child, subFiles, subDirs, err := fs.buildDirectoryTreeWithTotals(entryPath, maxDepth, currentDepth+1, followSymlinks)
			if err == nil {
				node.Children = append(node.Children, child)
				totalFiles += subFiles
				totalDirs += subDirs
			}
		} else {
			totalDirs++
		}
	}

	node.FileCount = int64(localFiles)
	return node, totalFiles, totalDirs, nil
}

// Internal helper for mcp error formatting
func errorResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.TextContent{Type: "text", Text: fmt.Sprintf("Error: %v", err)}},
		IsError: true,
	}
}
