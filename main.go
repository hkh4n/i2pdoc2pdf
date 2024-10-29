package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

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

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	inputDir := "./docs" // Adjust this to your docs directory
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
