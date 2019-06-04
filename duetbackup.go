package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

const (
	sysDir = "0:/sys"
)

var httpClient *http.Client

type localTime struct {
	Time time.Time
}

type file struct {
	Type string
	Name string
	Size uint64
	Date localTime `json:"date"`
}

type filelist struct {
	Dir   string
	Files []file
	next  uint64
}

func (lt *localTime) UnmarshalJSON(b []byte) (err error) {
	// Parse date string in local time (it does not provide any timezone information)
	lt.Time, err = time.ParseInLocation(`"2006-01-02T15:04:05"`, string(b), time.Local)
	return err
}

func getFileList(baseURL string, dir string, first uint64) (*filelist, error) {

	fileListURL := "rr_filelist?dir="

	resp, err := httpClient.Get(baseURL + fileListURL + dir)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var fl filelist
	err = json.Unmarshal(body, &fl)
	if err != nil {
		return nil, err
	}
	if fl.next > 0 {
		moreFiles, err := getFileList(baseURL, dir, fl.next)
		if err != nil {
			return nil, err
		}
		fl.Files = append(fl.Files, moreFiles.Files...)
	}

	// Sort folders first and by name
	sort.SliceStable(fl.Files, func(i, j int) bool {

		// Both same type so compare by name
		if fl.Files[i].Type == fl.Files[j].Type {
			return fl.Files[i].Name < fl.Files[j].Name
		}

		// Different types -> sort folders first
		return fl.Files[i].Type == "d"
	})
	return &fl, nil
}

func updateLocalFiles(baseURL string, fl *filelist, outDir string, removeLocal, verbose bool) error {

	fileDownloadURL := "rr_download?name=" + url.QueryEscape(fl.Dir+"/")

	for _, file := range fl.Files {
		fileName := filepath.Join(outDir, file.Name)
		fi, err := os.Stat(fileName)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		// It's a directory
		if file.Type == "d" {

			// Does not exist yet so try to create it
			if fi == nil {
				if verbose {
					log.Println("Adding new directory", fileName)
				}
				if err = os.MkdirAll(fileName, 0755); err != nil {
					return err
				}
			}

			// Go recursively into this directory
			if err = syncFolder(baseURL, fl.Dir+"/"+file.Name, fileName, removeLocal, verbose); err != nil {
				return err
			}
			continue
		}

		// File does not exist or is outdated so get it
		if fi == nil || fi.ModTime().Before(file.Date.Time) {
			if verbose {
				if fi != nil {
					log.Println("Updating", file.Name)
				} else {
					log.Println("Adding", file.Name)
				}
			}

			// Download file
			resp, err := httpClient.Get(baseURL + fileDownloadURL + url.QueryEscape(file.Name))
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}

			// Open or create corresponding local file
			nf, err := os.Create(fileName)
			if err != nil {
				return err
			}
			defer nf.Close()

			// Write contents to local file
			_, err = nf.Write(body)
			if err != nil {
				return err
			}

			// Adjust mtime
			os.Chtimes(fileName, file.Date.Time, file.Date.Time)
		} else {
			if verbose {
				log.Println(file.Name, "is up-to-date")
			}
		}

	}

	return nil
}

func removeDeletedFiles(fl *filelist, outDir string, verbose bool) error {

	existingFiles := make(map[string]struct{})
	for _, f := range fl.Files {
		existingFiles[f.Name] = struct{}{}
	}

	files, err := ioutil.ReadDir(outDir)
	if err != nil {
		return err
	}

	for _, f := range files {
		if _, exists := existingFiles[f.Name()]; !exists {
			if err := os.Remove(filepath.Join(outDir, f.Name())); err != nil {
				return err
			}
			if verbose {
				log.Println("Removed", f.Name())
			}
		}
	}

	return nil
}

func syncFolder(address, folder, outDir string, removeLocal, verbose bool) error {
	log.Println("Fetching filelist for", folder)
	fl, err := getFileList(address, url.QueryEscape(folder), 0)
	if err != nil {
		return err
	}

	log.Println("Downloading new/changed files from", folder, "to", outDir)
	if err = updateLocalFiles(address, fl, outDir, removeLocal, verbose); err != nil {
		return err
	}

	if removeLocal {
		log.Println("Removing no longer existing files in", outDir)
		if err = removeDeletedFiles(fl, outDir, verbose); err != nil {
			return err
		}
	}

	return nil
}

func getAddress(domain string, port uint64) string {
	return "http://" + domain + ":" + strconv.FormatUint(port, 10) + "/"
}

func connect(address, password string, verbose bool) error {
	if verbose {
		log.Println("Trying to connect to Duet")
	}
	path := "rr_connect?password=" + url.QueryEscape(password) + "&time=" + url.QueryEscape(time.Now().Format("2006-01-02T15:04:05"))
	_, err := httpClient.Get(address + path)
	return err
}

func main() {
	var domain, dirToBackup, outDir, password string
	var removeLocal, verbose bool
	var port uint64

	flag.StringVar(&domain, "domain", "", "Domain of Duet Wifi")
	flag.Uint64Var(&port, "port", 80, "Port of Duet Wifi")
	flag.StringVar(&dirToBackup, "dirToBackup", sysDir, "Directory on Duet to create a backup of")
	flag.StringVar(&outDir, "outDir", "", "Output dir of backup")
	flag.StringVar(&password, "password", "reprap", "Connection password")
	flag.BoolVar(&removeLocal, "removeLocal", false, "Remove files locally that have been deleted on the Duet")
	flag.BoolVar(&verbose, "verbose", false, "Output more details")
	flag.Parse()

	if domain == "" || outDir == "" {
		log.Println("domain and outDir are required")
		os.Exit(1)
	}

	if port > 65535 {
		log.Println("Invalid port", port)
		os.Exit(2)
	}

	tr := &http.Transport{DisableCompression: true}
	httpClient = &http.Client{Transport: tr}

	address := getAddress(domain, port)

	// Try to connect
	if err := connect(address, password, verbose); err != nil {
		log.Println("Duet currently not available")
		os.Exit(0)
	}

	// Get absolute path from user's input
	absPath, err := filepath.Abs(outDir)
	if err != nil {
		// Fall back to original user's input
		absPath = outDir
	}

	err = syncFolder(address, dirToBackup, absPath, removeLocal, verbose)
	if err != nil {
		log.Fatal(err)
	}
}
