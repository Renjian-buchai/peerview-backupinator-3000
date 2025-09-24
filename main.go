package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

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
	var srv *drive.Service
	{
		ctx := context.Background()
		b, err := os.ReadFile("credentials.json")
		if err != nil {
			log.Fatalf("Unable to read client secret file: %v", err)
		}

		// If modifying these scopes, delete your previously saved token.json.
		// If not, you'll end up searching for errors for 1 hour like I did
		config, err := google.ConfigFromJSON(b, drive.DriveReadonlyScope)
		if err != nil {
			log.Fatalf("Unable to parse client secret file to config: %v", err)
		}
		client := getClient(config)

		// Creating drive client so that you can create shit
		srv, err = drive.NewService(ctx, option.WithHTTPClient(client))
		if err != nil {
			log.Fatalf("Unable to retrieve Drive client: %v", err)
		}
	}

	// Json file containing the links to the data
	fileList := []([5]string){}
	{
		var files map[string]any

		jsonData, err := os.ReadFile("test.json")
		if err != nil {
			log.Fatalf("Unable to read peerview files")
		}

		err = json.Unmarshal([]byte(jsonData), &files)
		if err != nil {
			log.Fatalf("Unable to unmarshal json")
		}

		data := files["data"].([]interface{})

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
	}

	for _, item := range fileList {
		id, subject, title, filetype, link := item[0], item[1], item[2], item[3], item[4]
		// var pdf bool
		if filetype != "Google Document" {
			continue
		}

		var format string
		var file *http.Response
		{
			fileId := getFileID(link)

			fileMeta, err := srv.Files.Get(fileId).Do()
			if err != nil {
				log.Printf("Unable to open get file meta for file: %v. Err: %v", id, err)
				continue
			}

			if fileMeta.MimeType == "application/vnd.google-apps.document" {
				// This means that filetype is the native type, so you can export it as native type.
				file, err = srv.Files.Export(fileId, "application/pdf").Download()
				format = ".pdf"
			} else {
				// If you hit this case, you have to download as the original type
				file, err = srv.Files.Get(fileId).Download()
				format = mapMimeTypeToFormat(fileMeta.MimeType)
			}

			if err != nil {
				log.Printf("Unable to download file: %v. Err: %v", id, err)
				continue
			}
		}

		{
			// Create directories if necessary
			err := os.MkdirAll("output/"+subject+"/", os.ModePerm)
			if err != nil {
				log.Fatalf("Unable to create directories")
			}

			// Sanitise titles. I don't think got "/", but ehh
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

}
