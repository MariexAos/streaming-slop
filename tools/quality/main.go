// Command quality enforces the same nonblank, noncomment file limit as the frontend.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"strings"
)

func main() {
	if len(os.Args) == 3 && os.Args[1] == "integration" {
		if err := checkIntegration(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("Required database and media integration tests passed without skips.")
		return
	}

	if !checkFileSizes() {
		os.Exit(1)
	}
}

func checkFileSizes() bool {
	failed := false
	for _, root := range []string{"cmd", "internal", "tools"} {
		err := fs.WalkDir(os.DirFS(root), ".", func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			name := root + "/" + path
			lines, err := sourceLines(name)
			if err != nil {
				return err
			}
			if lines > 500 {
				fmt.Fprintf(os.Stderr, "%s: %d nonblank, noncomment lines exceeds 500\n", name, lines)
				failed = true
			}
			return nil
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			failed = true
		}
	}
	return !failed
}

func sourceLines(path string) (int, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, path, source, parser.ParseComments)
	if err != nil {
		return 0, err
	}
	if ast.IsGenerated(file) {
		return 0, nil
	}
	for _, group := range file.Comments {
		for _, comment := range group.List {
			start := positions.Position(comment.Pos()).Offset
			end := positions.Position(comment.End()).Offset
			for i := start; i < end; i++ {
				if source[i] != '\n' {
					source[i] = ' '
				}
			}
		}
	}
	count := 0
	for _, line := range strings.Split(string(source), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count, nil
}
