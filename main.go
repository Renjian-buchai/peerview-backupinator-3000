package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
)

func getDocsData(filename string) [][5]string {
	// Template for json
	var files map[string]any

	jsonData, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Unable to read peerview files")
	}

	err = json.Unmarshal([]byte(jsonData), &files)
	if err != nil {
		log.Fatalf("Unable to unmarshal json")
	}

	data := files["data"].([]interface{})

	fileList := [][5]string{}

	for _, item := range data {
		d := item.(map[string]interface{}) // cast to map

		id := d["id"].(string)
		title := d["title"].(string)
		filetype := d["type"].(string)
		link := d["link"].(string)
		subject, ok := d["subject"].(string)
		if !ok || subject == "" {
			subject = "Uncategorised"
		}

		fileList = append(fileList, [5]string{id, subject, title, filetype, link})
	}

	return fileList
}

func getFileID(link string) string {
	re := regexp.MustCompile(`/d/([a-zA-Z0-9_-]+)`)
	match := re.FindStringSubmatch(link)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func mapMimeTypeToFormat(mimeType string) string {
	switch mimeType {
	// Actual cancer
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"

	case "application/pdf":
		return ".pdf"

	default:
		log.Println("Unknown file type. MimeType:", mimeType)
		return ".bin"
	}
}

func sanitiseTitle(title string) string {
	// True for Windows, idk about linux, but I'm pretty sure that Windows is the more restrictive one.
	bannedChars := "/\\:*?\"<>|"
	for _, char := range bannedChars {
		title = strings.ReplaceAll(title, string(char), "")
	}
	return title
}

func main() {
	// Setting log file
	{
		// In theory, 0644 => rw to owner, read-only to everyone else.
		logFile, err := os.OpenFile("log.txt", os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			log.Printf("Unable to create log file")
		}
		defer logFile.Close()
		log.SetOutput(logFile)
	}

	// Read client secret file to identify google API thingy
	srv := authServer()

	// Read json containing data for notes, and output relevant data
	fileList := getDocsData("test.json")

	for _, item := range fileList {
		id, subject, title, filetype, link := item[0], item[1], item[2], item[3], item[4]
		// var pdf bool
		if filetype != "Google Document" {
			continue
		}

		fileId := getFileID(link)
		mimeType := identifyFile(srv, fileId)
		if mimeType == "" {
			continue
		}

		var format string

		var file *http.Response
		var err error

		if mimeType == "application/vnd.google-apps.document" {
			// This means that filetype is the native type, so you can export it as native type.
			file, err = srv.Files.Export(fileId, "application/pdf").Download()
			if err != nil {
				log.Printf("Unable to download file: %v. Err: %v", id, err)
				continue
			}

			format = ".pdf"
		} else {
			// If you hit this case, you have to download as the original type
			file, err = srv.Files.Get(fileId).Download()
			if err != nil {
				log.Printf("Unable to download file: %v. Err: %v", id, err)
				continue
			}

			format = mapMimeTypeToFormat(mimeType)
		}

		// Create directories if necessary
		err = os.MkdirAll("output/"+subject+"/", os.ModePerm)
		if err != nil {
			log.Fatalf("Unable to create directories")
		}

		// Sanitise titles. I don't think will have "/", but ehh
		filename := "output/" + subject + "/" + id + " - " + sanitiseTitle(title) + format

		output, err := os.Create(filename)
		if err != nil {
			log.Printf("Unable to create file: %v", (filename))
			continue
		}
		defer output.Close()

		// Need to copy and paste each byte from file's body data to the output
		_, err = io.Copy(output, file.Body)
		if err != nil {
			log.Printf("Unable to copy file: %v", filename)
			continue
		}
	}

}
