package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
)

func main() {
	http.HandleFunc("/", process)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	log.Printf("Listening on port %s", port)
	err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
	log.Fatal(err)
}

func process(w http.ResponseWriter, r *http.Request) {

	log.Println("Serving request")

	if r.Method == "GET" {
		fmt.Fprintln(w, "Ready to process POST requests from Cloud Storage trigger")
		return
	}

	//
	// Read request body containing GCS object metadata
	//
	gcsInputFile, err1 := readBody(r)
	if err1 != nil {
		log.Printf("Error reading POST data: %v", err1)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Problem with POST data: %v \n", err1)
		return
	}

	//
	// Working directory (concurrency-safe)
	//
	localDir, errDir := ioutil.TempDir("", "")
	if errDir != nil {
		log.Printf("Error creating local temp dir: %v", errDir)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Could not create a temp directory on server. \n")
		return
	}
	defer os.RemoveAll(localDir)

	//
	// Download input file from GCS
	//
	localInputFile, err2 := download(gcsInputFile, localDir)
	if err2 != nil {
		log.Printf("Error downloading GCS file [%s] from bucket [%s]: %v", gcsInputFile.Name, gcsInputFile.Bucket, err2)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error downloading GCS file [%s] from bucket [%s]", gcsInputFile.Name, gcsInputFile.Bucket)
		return
	}

	//
	// Use LibreOffice to convert local input file to local PDF file.
	//
	localPDFFilePath, err3 := convertToPDF(localInputFile.Name(), localDir, gcsInputFile.Name)
	if err3 != nil {
		log.Printf("Error converting to potree: %v", err3)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error converting to potree.")
		return
	}

	//
	// Upload the freshly generated PDF to GCS
	//

	//noExtName formats lidar.las -> lidar
	noExtName := gcsInputFile.Name[:(len(gcsInputFile.Name) - 4)]

	// change target Bucket to whatever dev diplay folder structure is
	targetBucket := os.Getenv("PDF_BUCKET")

	//html file upload
	err4 := upload((localPDFFilePath + "/" + noExtName + ".html"), targetBucket, "output/")
	if err4 != nil {
		log.Printf("Error uploading PDF file to bucket [%s]: %v", targetBucket, err4)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error downloading GCS file [%s] from bucket [%s]", gcsInputFile.Name, gcsInputFile.Bucket)
		return
	}
	//octree upload
	err7 := upload((localPDFFilePath + "/pointclouds/" + noExtName + "/octree.bin"), targetBucket, "output/pointclouds/" + noExtName + "/")
	if err7 != nil {
		log.Printf("Error uploading PDF file to bucket [%s]: %v", (targetBucket ), err7)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error downloading GCS file [%s] from bucket [%s]", gcsInputFile.Name, gcsInputFile.Bucket)
		return
	}
	//metadata.json upload
	err8 := upload((localPDFFilePath + "/pointclouds/" + noExtName + "/metadata.json"), targetBucket, "output/pointclouds/" + noExtName + "/" )
	if err8 != nil {
		log.Printf("Error uploading PDF file to bucket [%s]: %v", (targetBucket + "/pointclouds/" + noExtName + "/"), err8)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error downloading GCS file [%s] from bucket [%s]", gcsInputFile.Name, gcsInputFile.Bucket)
		return
	}
	// hieracrcy.bin upload
	err9 := upload((localPDFFilePath + "/pointclouds/" + noExtName + "/hierarchy.bin"), targetBucket, "output/pointclouds/" + noExtName + "/")
	if err9 != nil {
		log.Printf("Error uploading PDF file to bucket [%s]: %v", (targetBucket + "/pointclouds/" + noExtName + "/"), err9)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error downloading GCS file [%s] from bucket [%s]", gcsInputFile.Name, gcsInputFile.Bucket)
		return
	}

	//
	// Delete the original input file from GCS.
	//
	err5 := deleteGCSFile(gcsInputFile.Bucket, gcsInputFile.Name)
	if err5 != nil {
		log.Printf("Error deleting file [%s] from bucket [%s]: %v", gcsInputFile.Name, gcsInputFile.Bucket, err5)
		// This is not a blocking error. The PDF was successfully generated and uploaded.
	}

	log.Println("Successfully produced potree format")
	fmt.Fprintln(w, "Successfully produced potree format")
}

func convertToPDF(localFilePath string, localDir string, name string) (resultFilePath string, err error) {
	log.Printf("Converting [%s] to PDF", localFilePath)
	noExtName := name[:(len(name) - 4)]
	cmd := exec.Command("/opt/PotreeConverter/build/PotreeConverter", localFilePath, "-o", (localDir + "/output"),
		"-generate-page", noExtName)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	log.Println(cmd)
	err = cmd.Run()
	if err != nil {
		return "", err
	}

	return (localDir + "/output"), nil
}
