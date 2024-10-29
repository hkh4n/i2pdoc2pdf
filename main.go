package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/SebastiaanKlippert/go-wkhtmltopdf"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// findHTMLFiles returns a slice of HTML files, checking for index.html in directories
func findHTMLFiles(baseDir string) ([]string, error) {
	var files []string
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error accessing path %s: %v", path, err)
			return nil
		}

		// If it's a directory, look for index.html
		if info.IsDir() {
			indexPath := filepath.Join(path, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				log.Printf("Found index.html in directory: %s", path)
				files = append(files, indexPath)
			}
			return nil
		}

		// If it's a file with .html extension (but not index.html in subdirectories)
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".html") {
			dir := filepath.Dir(path)
			filename := filepath.Base(path)
			// Only include non-index.html files at the root level
			if dir == baseDir || filename != "index.html" {
				log.Printf("Found HTML file: %s", path)
				files = append(files, path)
			}
		}

		return nil
	})

	return files, err
}

// IGNORE THIS (notes): wget http://archive.ubuntu.com/ubuntu/pool/main/o/openssl/libssl1.1_1.1.1f-1ubuntu2.23_amd64.deb
// cleanupDownloadDir removes incomplete or failed downloads
func cleanupDownloadDir(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Remove temporary wget files
		if strings.HasSuffix(path, ".tmp") || strings.HasSuffix(path, ".wget") {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to remove temporary file %s: %w", path, err)
			}
		}
		return nil
	})
}

// RepositoryInfo holds information about the Git repository
type RepositoryInfo struct {
	URL      string // e.g., "https://github.com/username/i2p.www.git"
	Branch   string // e.g., "main"
	CloneDir string // Local directory to clone into
}

// ExecuteCommand runs a shell command and returns its output or an error
func ExecuteCommand(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the command and capture any errors
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %s %v, error: %v", name, args, err)
	}
	return nil
}

func CloneRepo(repo RepositoryInfo) error {
	// Ensure the clone directory exists
	if _, err := os.Stat(repo.CloneDir); os.IsNotExist(err) {
		err := os.MkdirAll(repo.CloneDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory %s: %v", repo.CloneDir, err)
		}
	}

	// Step 1: Initialize the Git repository
	fmt.Println("Initializing Git repository...")
	if err := ExecuteCommand(repo.CloneDir, "git", "init"); err != nil {
		return err
	}

	// Step 2: Add remote origin
	fmt.Println("Adding remote origin...")
	if err := ExecuteCommand(repo.CloneDir, "git", "remote", "add", "origin", repo.URL); err != nil {
		return err
	}

	// Step 5: Pull the specified branch
	fmt.Printf("Pulling branch '%s'...\n", repo.Branch)
	if err := ExecuteCommand(repo.CloneDir, "git", "pull", "origin", repo.Branch); err != nil {
		return err
	}

	fmt.Println("Sparse clone completed successfully.")
	return nil
}

func copyDir(source, destination string) {
	// Define the source and destination paths
	//source := "./i2p-www-docs/i2p2www/pages/site/docs"
	//destination := "./docs"

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		if _, err := os.Stat(destination); os.IsNotExist(err) {
			err := os.MkdirAll(destination, 0755)
			if err != nil {
				log.Fatalf("Failed to create destination directory: %v", err)
			}
		}
		cmd = exec.Command("robocopy", source, destination, "/E", "/COPYALL", "/MOVE", "/R:1", "/W:1")
		// Option 2: Using PowerShell's Copy-Item
		/*
			cmd = exec.Command("powershell", "-Command",
				fmt.Sprintf("Copy-Item -Path '%s' -Destination '%s' -Recurse -Force", source, destination))
		*/
	default:
		// Assume Unix-like system, use cp -r
		cmd = exec.Command("cp", "-r", source, destination)
	}

	// Set the standard output and error to the program's output
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Executing command: %v\n", cmd.Args)

	// Run the command
	err := cmd.Run()
	if err != nil {
		log.Fatalf("Command execution failed: %v", err)
	}

	fmt.Println("Directory copied successfully.")
}

// replaceURLForPlaceholders replaces {{ url_for('static', filename='path/to/image.png') }} with the relative path
func replaceURLForPlaceholders(doc *goquery.Document) {
	// Regular expression to match the url_for pattern
	re := regexp.MustCompile(`{{\s*url_for\(\s*'static'\s*,\s*filename\s*=\s*'([^']+)'\s*\)\s*}}`)

	// Find all img tags
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if exists {
			// Check if src matches the url_for pattern
			matches := re.FindStringSubmatch(src)
			if len(matches) == 2 {
				// matches[1] contains the filename
				newSrc := matches[1]
				s.SetAttr("src", newSrc)
				log.Printf("Replaced img src with relative path: %s", newSrc)
			}
		}
	})
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	// Get docs
	// Define the repository information
	repo := RepositoryInfo{
		URL:      "https://github.com/i2p/i2p.www.git",
		Branch:   "master",
		CloneDir: "i2p-www-docs", // Local directory name
	}

	// Get absolute path for CloneDir
	absPath, err := filepath.Abs(repo.CloneDir)
	if err != nil {
		log.Fatalf("Failed to get absolute path: %v", err)
	}
	repo.CloneDir = absPath

	// Start the sparse clone process
	// Check if the clone directory already exists
	if _, err := os.Stat(repo.CloneDir); os.IsNotExist(err) {
		// Directory does not exist, proceed to clone
		fmt.Printf("Repository directory '%s' does not exist. Starting clone...\n", repo.CloneDir)
		if err := CloneRepo(repo); err != nil {
			log.Fatalf("Failed to clone repository: %v", err)
		}
	} else {
		// Directory exists, skip cloning
		fmt.Printf("Repository directory '%s' already exists. Skipping clone.\n", repo.CloneDir)
	}
	/*
		if err := CloneSparseRepo(repo); err != nil {
			log.Fatalf("Failed to clone repository: %v", err)
		}

	*/

	fmt.Printf("The '%s' directory has been successfully cloned.")

	copyDir("./i2p-www-docs/i2p2www/pages/site/docs", "./docs")

	inputDir := "./docs"
	outputFile := "i2p-documentation.pdf"

	// Find all HTML files
	htmlFiles, err := findHTMLFiles(inputDir)
	if err != nil {
		log.Fatalf("Error finding HTML files: %v", err)
	}

	if len(htmlFiles) == 0 {
		log.Fatal("No HTML files found in directory")
	}

	log.Printf("Found %d HTML files to process", len(htmlFiles))

	// Create combined HTML document
	combinedHTML := strings.Builder{}
	combinedHTML.WriteString(`
	<!DOCTYPE html>
	<html>
	<head>
		<meta charset="UTF-8">
		<title>I2P Documentation</title>
		<style>
			body { 
				font-family: Arial, sans-serif;
				max-width: 800px;
				margin: 0 auto;
				padding: 20px;
			}
			.page-break { 
				page-break-after: always;
				height: 1px;
			}
			.chapter { 
				margin-top: 30px;
			}
			pre {
				background-color: #f5f5f5;
				padding: 10px;
				border-radius: 5px;
				overflow-x: auto;
			}
			code {
				font-family: monospace;
			}
		</style>
	</head>
	<body>
	<h1>I2P Documentation</h1>
	<div class="page-break"></div>
`)

	// Add table of contents
	combinedHTML.WriteString("<h2>Table of Contents</h2><ul>")
	for _, htmlFile := range htmlFiles {
		// Create readable section name from file path
		sectionName := strings.TrimPrefix(htmlFile, inputDir)
		sectionName = strings.TrimPrefix(sectionName, "/")
		sectionName = strings.TrimSuffix(sectionName, "/index.html")
		sectionName = strings.TrimSuffix(sectionName, ".html")
		sectionName = strings.ReplaceAll(sectionName, "/", " → ")
		combinedHTML.WriteString(fmt.Sprintf("<li>%s</li>", sectionName))
	}
	combinedHTML.WriteString("</ul><div class=\"page-break\"></div>")

	// Process each HTML file
	for _, htmlFile := range htmlFiles {
		log.Printf("Processing %s", htmlFile)

		content, err := ioutil.ReadFile(htmlFile)
		if err != nil {
			log.Printf("Error reading file %s: %v", htmlFile, err)
			continue
		}

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(content)))
		if err != nil {
			log.Printf("Error parsing HTML from %s: %v", htmlFile, err)
			continue
		}

		// Clean up HTML
		doc.Find("script").Remove()
		doc.Find("style").Remove()
		doc.Find("link").Remove()
		doc.Find("meta").Remove()
		doc.Find("iframe").Remove()
		doc.Find("noscript").Remove()

		// Replace url_for placeholders in img src attributes
		replaceURLForPlaceholders(doc)

		// Extract the body content
		bodyContent := doc.Find("body").First()
		if bodyContent.Length() > 0 {
			// Create section title from file path
			sectionName := strings.TrimPrefix(htmlFile, inputDir)
			sectionName = strings.TrimPrefix(sectionName, "/")
			sectionName = strings.TrimSuffix(sectionName, "/index.html")
			sectionName = strings.TrimSuffix(sectionName, ".html")
			sectionName = strings.ReplaceAll(sectionName, "/", " → ")

			// Get HTML content and handle potential error
			htmlContent, err := bodyContent.Html()
			if err != nil {
				log.Printf("Error getting HTML content from %s: %v", htmlFile, err)
				continue
			}

			combinedHTML.WriteString(fmt.Sprintf(`
				<div class="chapter">
					<h2>%s</h2>
					%s
					<div class="page-break"></div>
				</div>
			`, sectionName, htmlContent))
		}
	}

	combinedHTML.WriteString("</body></html>")

	// Write combined HTML to file
	tempFile := "combined.html"
	err = ioutil.WriteFile(tempFile, []byte(combinedHTML.String()), 0644)
	if err != nil {
		log.Fatalf("Error writing combined HTML: %v", err)
	}
	defer os.Remove(tempFile)

	// Initialize PDF generator
	pdfg, err := wkhtmltopdf.NewPDFGenerator()
	if err != nil {
		log.Fatalf("Failed to create PDF generator: %v", err)
	}

	// Configure PDF settings
	pdfg.Dpi.Set(96)
	pdfg.MarginBottom.Set(20)
	pdfg.MarginTop.Set(20)
	pdfg.MarginLeft.Set(20)
	pdfg.MarginRight.Set(20)
	pdfg.Orientation.Set(wkhtmltopdf.OrientationPortrait)
	pdfg.PageSize.Set(wkhtmltopdf.PageSizeA4)

	// Create page from combined HTML
	page := wkhtmltopdf.NewPage(tempFile)
	page.EnableLocalFileAccess.Set(true)
	page.LoadErrorHandling.Set("ignore")
	//page.EnableJavascript.Set(false)
	page.LoadMediaErrorHandling.Set("ignore")
	page.HeaderRight.Set("[page]/[toPage]")

	pdfg.AddPage(page)

	// Generate PDF
	log.Println("Generating PDF...")
	err = pdfg.Create()
	if err != nil {
		log.Fatalf("Error creating PDF: %v", err)
	}

	// Write to file
	log.Printf("Writing PDF to %s", outputFile)
	err = pdfg.WriteFile(outputFile)
	if err != nil {
		log.Fatalf("Error writing PDF: %v", err)
	}

	log.Println("PDF generation complete!")
}
