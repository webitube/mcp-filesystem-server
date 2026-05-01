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
	path, err := request.RequireString("starting_path")
	if err != nil {
		return nil, err
	}

	// Handle empty or relative paths like "." or "./" by converting to absolute path
	if path == "." || path == "./" {
		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error resolving current directory: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
		path = cwd
	}

	// Extract depth parameter (optional, default: 3)
	depth := 3 // Default value
	if depthParam, err := request.RequireFloat("max_depth"); err == nil {
		depth = int(depthParam)
	}

	// Extract follow_symlinks parameter (optional, default: false)
	followSymlinks := false // Default value
	if followParam, err := request.RequireBool("follow_symlinks"); err == nil {
		followSymlinks = followParam
	}

	// Validate the path is within allowed directories
	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Check if it's a directory
	info, err := os.Stat(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	if !info.IsDir() {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "Error: The specified path is not a directory",
				},
			},
			IsError: true,
		}, nil
	}

	// Build the file tree structure
	tree, err := fs.buildFileTree(validPath, depth, 0, followSymlinks)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error building directory tree: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Convert to JSON
	jsonData, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error generating JSON: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Create resource URI for the directory
	resourceURI := pathToResourceURI(validPath)

	// Return the result
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Directory tree for %s (max depth: %d):\n\n%s", validPath, depth, string(jsonData)),
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

// buildFileTree builds a tree representation of the filesystem starting at the given path
func (fs *FilesystemHandler) buildFileTree(path string, maxDepth int, currentDepth int, followSymlinks bool) (*FileNode, error) {
	// Validate the path
	validPath, err := fs.validatePath(path)
	if err != nil {
		return nil, err
	}

	// Get file info
	info, err := os.Stat(validPath)
	if err != nil {
		return nil, err
	}

	// Create the node
	node := &FileNode{
		Name:     filepath.Base(validPath),
		Path:     validPath,
		Modified: info.ModTime(),
	}

	// Set type and size
	if info.IsDir() {
		node.Type = "directory"

		// If we haven't reached the max depth, process children
		if currentDepth < maxDepth {
			// Read directory entries
			entries, err := os.ReadDir(validPath)
			if err != nil {
				return nil, err
			}

			// Process each entry
			for _, entry := range entries {
				entryPath := filepath.Join(validPath, entry.Name())

				// Handle symlinks
				if entry.Type()&os.ModeSymlink != 0 {
					if !followSymlinks {
						// Skip symlinks if not following them
						continue
					}

					// Resolve symlink
					linkDest, err := filepath.EvalSymlinks(entryPath)
					if err != nil {
						// Skip invalid symlinks
						continue
					}

					// Validate the symlink destination is within allowed directories
					if !fs.isPathInAllowedDirs(linkDest) {
						// Skip symlinks pointing outside allowed directories
						continue
					}

					entryPath = linkDest
				}

				// Recursively build child node
				childNode, err := fs.buildFileTree(entryPath, maxDepth, currentDepth+1, followSymlinks)
				if err != nil {
					// Skip entries with errors
					continue
				}

				// Add child to the current node
				node.Children = append(node.Children, childNode)
			}
		}
	} else {
		node.Type = "file"
		node.Size = info.Size()
	}

	return node, nil
}

func (fs *FilesystemHandler) HandleDirectoryTree(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	path, err := request.RequireString("starting_path")
	if err != nil {
		return nil, err
	}

	// Handle empty or relative paths like "." or "./" by converting to absolute path
	if path == "." || path == "./" {
		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("Error resolving current directory: %v", err),
					},
				},
				IsError: true,
			}, nil
		}
		path = cwd
	}

	// Extract depth parameter (optional, default: 3)
	depth := 3 // Default value
	if depthParam, err := request.RequireFloat("max_depth"); err == nil {
		depth = int(depthParam)
	}

	// Extract follow_symlinks parameter (optional, default: false)
	followSymlinks := false // Default value
	if followParam, err := request.RequireBool("follow_symlinks"); err == nil {
		followSymlinks = followParam
	}

	// Validate the path is within allowed directories
	validPath, err := fs.validatePath(path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Check if it's a directory
	info, err := os.Stat(validPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	if !info.IsDir() {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: The specified path is not a directory. path=%s", path),
				},
			},
			IsError: true,
		}, nil
	}

	// Build the directoryTree structure
	fileTree, err := fs.buildDirectoryTree(validPath, depth, 0, followSymlinks)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error building directory tree: %v: validPath=%s, depth=%d, followSymLinks=%t", err, validPath, depth, followSymlinks),
				},
			},
			IsError: true,
		}, nil
	}

	// Convert to JSON
	jsonData, err := json.MarshalIndent(fileTree, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error generating JSON: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Create resource URI for the directory
	resourceURI := pathToResourceURI(validPath)

	// Return the result
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Directory tree for %s (max depth: %d):\n\n%s", validPath, depth, string(jsonData)),
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

// buildTree creates a hierarchical view of directories only, including file counts.
func (fs *FilesystemHandler) buildDirectoryTree(path string, maxDepth int, currentDepth int, followSymlinks bool) (*DirectoryNode, error) {
	// Validate the path using existing handler logic
	validPath, err := fs.validatePath(path)
	if err != nil {
		return nil, err
	}

	// Get info for the current path
	info, err := os.Stat(validPath)
	if err != nil {
		return nil, err
	}

	// If the root provided isn't a directory, we can't build a directory tree
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", validPath)
	}

	// Initialize the node for the current directory
	node := &DirectoryNode{
		Name:     filepath.Base(validPath),
		Path:     validPath,
		Children: []*DirectoryNode{},
	}

	// Read directory entries to count files and find sub-directories
	entries, err := os.ReadDir(validPath)
	if err != nil {
		return nil, err
	}

	fileCount := 0
	for _, entry := range entries {
		entryPath := filepath.Join(validPath, entry.Name())
		isDir := entry.IsDir()

		// Handle symlink resolution if enabled
		if entry.Type()&os.ModeSymlink != 0 && followSymlinks {
			resolvedPath, err := filepath.EvalSymlinks(entryPath)
			if err == nil && fs.isPathInAllowedDirs(resolvedPath) {
				linkInfo, err := os.Stat(resolvedPath)
				if err == nil {
					isDir = linkInfo.IsDir()
					entryPath = resolvedPath
				}
			}
		}

		if !isDir {
			// Increment count for files
			fileCount++
		} else if currentDepth < maxDepth {
			// Recursively process sub-directories if depth permits
			childNode, err := fs.buildDirectoryTree(entryPath, maxDepth, currentDepth+1, followSymlinks)
			if err == nil && childNode != nil {
				node.Children = append(node.Children, childNode)
			}
		}
	}

	node.FileCount = int64(fileCount)
	return node, nil
}
