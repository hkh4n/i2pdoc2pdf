package main

import (
	"context"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/SebastiaanKlippert/go-wkhtmltopdf"
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

type DownloadConfig struct {
	URL            string
	OutputDir      string
	MaxDepth       int
	WaitSeconds    int
	RateLimit      string
	ConvertScript  string
	TimeoutMinutes int
}

func downloadAndProcessDocs(config DownloadConfig) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Change to output directory
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	if err := os.Chdir(config.OutputDir); err != nil {
		return fmt.Errorf("failed to change to output directory: %w", err)
	}
	defer os.Chdir(originalDir)

	// Prepare wget command
	// Modified wget arguments to strictly target docs directory
	args := []string{
		"--recursive",
		"--no-clobber",
		"--page-requisites",
		"--html-extension",
		"--convert-links",
		"--restrict-file-names=windows",
		"--domains", "geti2p.net",
		"--no-parent",
		// Include only the docs directory
		"--include-directories=/en/docs",
		// Explicitly exclude other directories
		"--exclude-directories=/en/get-involved,/en/blog,/en/research,/en/comparison,/en/about",
		// Reject specific file patterns
		"--reject=*get-involved*,*blog*,*research*,*comparison*,*about*",
		fmt.Sprintf("--wait=%d", config.WaitSeconds),
		fmt.Sprintf("--limit-rate=%s", config.RateLimit),
		fmt.Sprintf("-l %d", config.MaxDepth),
		config.URL,
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(config.TimeoutMinutes)*time.Minute)
	defer cancel()

	// Run wget command
	fmt.Println("Starting wget download...")
	cmd := exec.CommandContext(ctx, "wget", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wget command failed: %w", err)
	}

	// Find the downloaded directory
	downloadedDir := filepath.Join(config.OutputDir, "geti2p.net")
	if _, err := os.Stat(downloadedDir); err != nil {
		return fmt.Errorf("downloaded directory not found: %w", err)
	}

	// Run convert script
	if config.ConvertScript != "" {
		fmt.Println("Running conversion script...")
		convertCmd := exec.CommandContext(ctx, "bash", config.ConvertScript, downloadedDir)
		convertCmd.Stdout = os.Stdout
		convertCmd.Stderr = os.Stderr

		if err := convertCmd.Run(); err != nil {
			return fmt.Errorf("conversion script failed: %w", err)
		}
	}

	return nil
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

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	// Get docs

	config := DownloadConfig{
		URL:            "https://geti2p.net/en/docs",
		OutputDir:      "./i2p-docs",
		MaxDepth:       3,
		WaitSeconds:    1,
		RateLimit:      "200k",
		ConvertScript:  "./convert.sh",
		TimeoutMinutes: 30,
	}
	// Create a channel for handling interrupts
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	// Create error channel
	errChan := make(chan error, 1)

	// Run download in goroutine
	go func() {
		errChan <- downloadAndProcessDocs(config)
	}()

	// Wait for either completion or interrupt
	select {
	case err := <-errChan:
		if err != nil {
			log.Printf("Error during download and conversion: %v", err)
			// Attempt cleanup
			if cleanErr := cleanupDownloadDir(config.OutputDir); cleanErr != nil {
				log.Printf("Error during cleanup: %v", cleanErr)
			}
			os.Exit(1)
		}
		fmt.Println("Download and conversion completed successfully!")

	case <-interrupt:
		fmt.Println("\nReceived interrupt signal. Cleaning up...")
		if err := cleanupDownloadDir(config.OutputDir); err != nil {
			log.Printf("Error during cleanup: %v", err)
		}
		os.Exit(1)
	}
	os.Exit(0)

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
