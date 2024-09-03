package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var minioClient *minio.Client

func init() {
	var err error
	minioClient, err = minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("admin", "admin123", ""),
		Secure: false,
	})
	if err != nil {
		log.Fatalf("Failed to create MinIO client: %v", err)
	}
}

func generateUniqueFileName(originalFileName string) string {
	extension := filepath.Ext(originalFileName)
	baseName := filepath.Base(originalFileName[:len(originalFileName)-len(extension)])
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s_%d%s", baseName, timestamp, extension)
}

func uploadFileHandler(w http.ResponseWriter, r *http.Request) {
	bucketName := "cloud1"

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["file"]
	if files == nil {
		http.Error(w, "No files uploaded", http.StatusBadRequest)
		return
	}

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, "Unable to get file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		_, err = minioClient.PutObject(context.Background(), bucketName, fileHeader.Filename, file, -1, minio.PutObjectOptions{})
		if err != nil {
			http.Error(w, "Unable to upload file", http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func listFilesHandler(w http.ResponseWriter, r *http.Request) {
	bucketName := "cloud1"
	objects := minioClient.ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{Recursive: true})

	var fileNames []string
	for object := range objects {
		if object.Err != nil {
			http.Error(w, "Unable to list files", http.StatusInternalServerError)
			return
		}
		fileNames = append(fileNames, object.Key)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fileNames)
}

func downloadFileHandler(w http.ResponseWriter, r *http.Request) {
	bucketName := "cloud1"
	fileName := r.URL.Query().Get("file")

	if fileName == "" {
		http.Error(w, "File name is required", http.StatusBadRequest)
		return
	}

	obj, err := minioClient.GetObject(context.Background(), bucketName, fileName, minio.GetObjectOptions{})
	if err != nil {
		http.Error(w, "Unable to get file", http.StatusInternalServerError)
		return
	}
	defer obj.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	w.Header().Set("Content-Type", "application/octet-stream")
	io.Copy(w, obj)
}

func viewFileHandler(w http.ResponseWriter, r *http.Request) {
	bucketName := "cloud1"
	fileName := r.URL.Query().Get("file")

	if fileName == "" {
		http.Error(w, "File name is required", http.StatusBadRequest)
		return
	}

	obj, err := minioClient.GetObject(context.Background(), bucketName, fileName, minio.GetObjectOptions{})
	if err != nil {
		http.Error(w, "Unable to get file", http.StatusInternalServerError)
		return
	}
	defer obj.Close()

	w.Header().Set("Content-Type", "image/jpeg")
	io.Copy(w, obj)
}

func deleteFileHandler(w http.ResponseWriter, r *http.Request) {
	bucketName := "cloud1"
	fileName := r.URL.Query().Get("file")

	if fileName == "" {
		http.Error(w, "File name is required", http.StatusBadRequest)
		return
	}

	err := minioClient.RemoveObject(context.Background(), bucketName, fileName, minio.RemoveObjectOptions{})
	if err != nil {
		http.Error(w, "Unable to delete file", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func serveFrontend(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", serveFrontend).Methods("GET")
	r.HandleFunc("/upload", uploadFileHandler).Methods("POST")
	r.HandleFunc("/list-files", listFilesHandler).Methods("GET")
	r.HandleFunc("/download-file", downloadFileHandler).Methods("GET")
	r.HandleFunc("/view-file", viewFileHandler).Methods("GET")
	r.HandleFunc("/delete-file", deleteFileHandler).Methods("DELETE")

	http.Handle("/", r)
	fmt.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
