package duetbackup

import (
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
	SysDir    = "0:/sys"
	dirMarker = ".duetbackup"
)

var multiSlashRegex = regexp.MustCompile(`/{2,}`)

// CleanPath will reduce multiple consecutive slashes into one and
// then remove a trailing slash if any.
func CleanPath(path string) string {
	cleanedPath := multiSlashRegex.ReplaceAllString(path, "/")
	cleanedPath = strings.TrimSuffix(cleanedPath, "/")
	return cleanedPath
}

type Duetbackup interface {
	// SyncFolder will syncrhonize the contents of a remote folder to a local directory.
	// The boolean flag removeLocal decides whether or not files that have been remove
	// remote should also be deleted locally
	SyncFolder(remoteFolder, outDir string, excls Excludes, removeLocal bool) error
}

type duetbackup struct {
	rfm     rrffm.RRFFileManager
	verbose bool
}

func New(rfm rrffm.RRFFileManager, verbose bool) Duetbackup {
	return &duetbackup{
		rfm:     rfm,
		verbose: verbose,
	}
}

// ensureOutDirExists will create the local directory if it does not exist
// and will in any case create the marker file inside it
func (d *duetbackup) ensureOutDirExists(outDir string) error {
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
		if d.verbose {
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

func (d *duetbackup) updateLocalFiles(fl *rrffm.Filelist, outDir string, excls Excludes, removeLocal bool) error {

	if err := d.ensureOutDirExists(outDir); err != nil {
		return err
	}

	for _, file := range fl.Files {
		if file.IsDir() {
			continue
		}
		remoteFilename := fmt.Sprintf("%s/%s", fl.Dir, file.Name)

		// Skip files covered by an exclude pattern
		if excls.Contains(remoteFilename) {
			if d.verbose {
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

			// Download file
			body, duration, err := d.rfm.Download(remoteFilename)
			if err != nil {
				return err
			}
			if d.verbose {
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
			if d.verbose {
				log.Println("  Up-to-date:", remoteFilename)
			}
		}

	}

	return nil
}

// isManagedDirectory checks wether the given path is a directory and
// if so if it contains the marker file. It will return false in case
// any error has occured.
func (d *duetbackup) isManagedDirectory(basePath string, f os.FileInfo) bool {
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

func (d *duetbackup) removeDeletedFiles(fl *rrffm.Filelist, outDir string) error {

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
			if !d.isManagedDirectory(outDir, f) || f.Name() == dirMarker {
				continue
			}
			if err := os.RemoveAll(filepath.Join(outDir, f.Name())); err != nil {
				return err
			}
			if d.verbose {
				log.Println("  Removed:   ", f.Name())
			}
		}
	}

	return nil
}

func (d *duetbackup) SyncFolder(folder, outDir string, excls Excludes, removeLocal bool) error {

	// Skip complete directories if they are covered by an exclude pattern
	if excls.Contains(folder) {
		log.Println("Excluding", folder)
		return nil
	}

	log.Println("Fetching filelist for", folder)
	fl, err := d.rfm.Filelist(folder)
	if err != nil {
		return err
	}

	log.Println("Downloading new/changed files from", folder, "to", outDir)
	if err = d.updateLocalFiles(fl, outDir, excls, removeLocal); err != nil {
		return err
	}

	if removeLocal {
		log.Println("Removing no longer existing files in", outDir)
		if err = d.removeDeletedFiles(fl, outDir); err != nil {
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
		if err = d.SyncFolder(remoteFilename, fileName, excls, removeLocal); err != nil {
			return err
		}
	}

	return nil
}
