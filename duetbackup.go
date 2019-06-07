package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/wilriker/rrffm"
)

const (
	sysDir    = "0:/sys"
	dirMarker = ".duetbackup"
)

var rfm rrffm.RRFFileManager
var multiSlashRegex = regexp.MustCompile(`/{2,}`)

type excludes struct {
	excls []string
}

func (e *excludes) String() string {
	return strings.Join(e.excls, ",")
}

func (e *excludes) Set(value string) error {
	e.excls = append(e.excls, cleanPath(value))
	return nil
}

// Contains checks if the given path starts with any of the known excludes
func (e *excludes) Contains(path string) bool {
	for _, excl := range e.excls {
		if strings.HasPrefix(path, excl) {
			return true
		}
	}
	return false
}

// cleanPath will reduce multiple consecutive slashes into one and
// then remove a trailing slash if any.
func cleanPath(path string) string {
	cleanedPath := multiSlashRegex.ReplaceAllString(path, "/")
	cleanedPath = strings.TrimSuffix(cleanedPath, "/")
	return cleanedPath
}

// ensureOutDirExists will create the local directory if it does not exist
// and will in any case create the marker file inside it
func ensureOutDirExists(outDir string, verbose bool) error {
	path, err := filepath.Abs(outDir)
	if err != nil {
		return err
	}

	// Check if the directory exists
	fi, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Create the directory
	if fi == nil {
		if verbose {
			log.Println("  Creating directory", path)
		}
		if err = os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}

	// Create the marker file
	markerFile, err := os.Create(filepath.Join(path, dirMarker))
	if err != nil {
		return err
	}
	markerFile.Close()

	return nil
}

func updateLocalFiles(fl *rrffm.Filelist, outDir string, excls excludes, removeLocal, verbose bool) error {

	if err := ensureOutDirExists(outDir, verbose); err != nil {
		return err
	}

	for _, file := range fl.Files {
		if file.IsDir() {
			continue
		}
		remoteFilename := fmt.Sprintf("%s/%s", fl.Dir, file.Name)

		// Skip files covered by an exclude pattern
		if excls.Contains(remoteFilename) {
			if verbose {
				log.Println("  Excluding: ", remoteFilename)
			}
			continue
		}

		fileName := filepath.Join(outDir, file.Name)
		fi, err := os.Stat(fileName)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		// File does not exist or is outdated so get it
		if fi == nil || fi.ModTime().Before(file.Date()) {
			if verbose {
			}

			// Download file
			body, duration, err := rfm.Download(remoteFilename)
			if err != nil {
				return err
			}
			if verbose {
				kibs := (float64(file.Size) / duration.Seconds()) / 1024
				if fi != nil {
					log.Printf("  Updated:   %s (%.1f KiB/s)", remoteFilename, kibs)
				} else {
					log.Printf("  Added:     %s (%.1f KiB/s)", remoteFilename, kibs)
				}
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
			os.Chtimes(fileName, file.Date(), file.Date())
		} else {
			if verbose {
				log.Println("  Up-to-date:", remoteFilename)
			}
		}

	}

	return nil
}

// isManagedDirectory checks wether the given path is a directory and
// if so if it contains the marker file. It will return false in case
// any error has occured.
func isManagedDirectory(basePath string, f os.FileInfo) bool {
	if !f.IsDir() {
		return false
	}
	markerFile := filepath.Join(basePath, f.Name(), dirMarker)
	fi, err := os.Stat(markerFile)
	if err != nil && !os.IsNotExist(err) {
		return false
	}
	if fi == nil {
		return false
	}
	return true
}

func removeDeletedFiles(fl *rrffm.Filelist, outDir string, verbose bool) error {

	// Pseudo hash-set of known remote filenames
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

			// Skip directories not managed by us as well as our marker file
			if !isManagedDirectory(outDir, f) || f.Name() == dirMarker {
				continue
			}
			if err := os.RemoveAll(filepath.Join(outDir, f.Name())); err != nil {
				return err
			}
			if verbose {
				log.Println("  Removed:   ", f.Name())
			}
		}
	}

	return nil
}

func syncFolder(folder, outDir string, excls excludes, removeLocal, verbose bool) error {

	// Skip complete directories if they are covered by an exclude pattern
	if excls.Contains(folder) {
		log.Println("Excluding", folder)
		return nil
	}

	log.Println("Fetching filelist for", folder)
	fl, err := rfm.Filelist(folder)
	if err != nil {
		return err
	}

	log.Println("Downloading new/changed files from", folder, "to", outDir)
	if err = updateLocalFiles(fl, outDir, excls, removeLocal, verbose); err != nil {
		return err
	}

	if removeLocal {
		log.Println("Removing no longer existing files in", outDir)
		if err = removeDeletedFiles(fl, outDir, verbose); err != nil {
			return err
		}
	}

	// Traverse into subdirectories
	for _, file := range fl.Files {
		if !file.IsDir() {
			continue
		}
		remoteFilename := fmt.Sprintf("%s/%s", fl.Dir, file.Name)
		fileName := filepath.Join(outDir, file.Name)
		if err = syncFolder(remoteFilename, fileName, excls, removeLocal, verbose); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	var domain, dirToBackup, outDir, password string
	var removeLocal, verbose bool
	var port uint64
	var excls excludes

	flag.StringVar(&domain, "domain", "", "Domain of Duet Wifi")
	flag.Uint64Var(&port, "port", 80, "Port of Duet Wifi")
	flag.StringVar(&dirToBackup, "dirToBackup", sysDir, "Directory on Duet to create a backup of")
	flag.StringVar(&outDir, "outDir", "", "Output dir of backup")
	flag.StringVar(&password, "password", "reprap", "Connection password")
	flag.BoolVar(&removeLocal, "removeLocal", false, "Remove files locally that have been deleted on the Duet")
	flag.BoolVar(&verbose, "verbose", false, "Output more details")
	flag.Var(&excls, "exclude", "Exclude paths starting with this string (can be passed multiple times)")
	flag.Parse()

	if domain == "" || outDir == "" {
		log.Fatal("-domain and -outDir are mandatory parameters")
	}

	if port > 65535 {
		log.Fatal("Invalid port", port)
	}

	rfm = rrffm.New(domain, port)

	// Try to connect
	if verbose {
		log.Println("Trying to connect to Duet")
	}
	if err := rfm.Connect(password); err != nil {
		log.Fatal(err)
		log.Println("Duet currently not available")
		os.Exit(0)
	}

	// Get absolute path from user's input
	absPath, err := filepath.Abs(outDir)
	if err != nil {
		// Fall back to original user's input
		absPath = outDir
	}

	if err = syncFolder(cleanPath(dirToBackup), absPath, excls, removeLocal, verbose); err != nil {
		log.Fatal(err)
	}
}
