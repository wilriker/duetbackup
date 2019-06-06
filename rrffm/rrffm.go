package rrffm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"
)

const (
	connectURL      = "%s/rr_connect?password=%s&time=%s"
	fileListURL     = "%s/rr_filelist?dir=%s"
	fileDownloadURL = "%s/rr_download?name=%s"
	mkdirURL        = "%s/rr_mkdir?dir=%s"
	moveURL         = "%s/rr_move?old=%s&new=%s"
	uploadURL       = "%s/rr_upload?name=%s&time=%s"
	deleteURL       = "%s/rr_delete?name=%s"
	typeDirectory   = "d"
	typeFile        = "f"
	noErrorResponse = `{"err":0}`
	// TimeFormat is the format of timestamps used by RRF
	TimeFormat = "2006-01-02T15:04:05"
)

// RRFFileManager provides means to interact with SD card contents on a machine
// using RepRapFirmware (RRF). It will communicate through its HTTP interface.
type RRFFileManager interface {
	// Connect establishes a connection to RepRapFirmware
	Connect(password string) error

	// GetFilelist will download a list of all files (also including directories) for the given path
	GetFilelist(path string) (*Filelist, error)

	// GetFile downloads a file with the given path also returning the duration of this action
	GetFile(filepath string) ([]byte, *time.Duration, error)

	// Mkdir creates a new directory with the given path
	Mkdir(path string) error

	// Move renames or moves a file or directory (only within the same SD card)
	Move(oldpath, newpath string) error

	// MoveOverwrite will delete the target file first and thus overwriting it
	MoveOverwrite(oldpath, newpath string) error

	// Delete removes the given path. It will fail for non-empty directories.
	Delete(path string) error

	// DeleteRecursive removes the given path recursively. This will also delete directories with all their contents.
	DeleteRecursive(path string) error

	// Upload uploads a new file to the given path on the SD card
	Upload(path string, content []byte) (*time.Duration, error)
}

type rrffm struct {
	httpClient *http.Client
	baseURL    string
}

// New creates a new instance of RRFFileManager
func New(domain string, port uint64) RRFFileManager {
	tr := &http.Transport{DisableCompression: true}
	return &rrffm{
		httpClient: &http.Client{Transport: tr},
		baseURL:    "http://" + domain + ":" + strconv.FormatUint(port, 10),
	}
}

func (rrffm *rrffm) getTimestamp(time time.Time) string {
	return time.Format(TimeFormat)
}

func (rrffm *rrffm) Connect(password string) error {
	_, _, err := rrffm.doGetRequest(fmt.Sprintf(connectURL, rrffm.baseURL, url.QueryEscape(password), url.QueryEscape(rrffm.getTimestamp(time.Now()))))
	return err
}

func (rrffm *rrffm) GetFilelist(dir string) (*Filelist, error) {
	return rrffm.getFileListRecursively(url.QueryEscape(dir), 0)
}

func (rrffm *rrffm) getFileListRecursively(dir string, first uint64) (*Filelist, error) {

	body, _, err := rrffm.doGetRequest(fmt.Sprintf(fileListURL, rrffm.baseURL, dir))
	if err != nil {
		return nil, err
	}

	var fl Filelist
	err = json.Unmarshal(body, &fl)
	if err != nil {
		return nil, err
	}

	// If the response signals there is more to fetch do it recursively
	if fl.next > 0 {
		moreFiles, err := rrffm.getFileListRecursively(dir, fl.next)
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
		return fl.Files[i].Type == typeDirectory
	})
	return &fl, nil
}

func (rrffm *rrffm) GetFile(filepath string) ([]byte, *time.Duration, error) {
	return rrffm.doGetRequest(fmt.Sprintf(fileDownloadURL, rrffm.baseURL, url.QueryEscape(filepath)))
}

// download will perform a GET request on the given URL and return
// the content of the response, a duration on how long it took (including
// setup of connection) or an error in case something went wrong
func (rrffm *rrffm) doGetRequest(url string) ([]byte, *time.Duration, error) {
	start := time.Now()
	resp, err := rrffm.httpClient.Get(url)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	duration := time.Since(start)
	if err != nil {
		return nil, nil, err
	}
	return body, &duration, nil
}

func (rrffm *rrffm) doPostRequest(url string, content []byte) ([]byte, *time.Duration, error) {
	start := time.Now()
	resp, err := rrffm.httpClient.Post(url, "text/plain", bytes.NewReader(content))
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	duration := time.Since(start)
	if err != nil {
		return nil, nil, err
	}
	return body, &duration, nil
}

func (rrffm *rrffm) checkError(action string, resp []byte, err error) error {
	if err != nil {
		return err
	}
	if string(resp) != noErrorResponse {
		return errors.New("Failed to perform: " + action)
	}
	return nil
}

func (rrffm *rrffm) Mkdir(path string) error {
	resp, _, err := rrffm.doGetRequest(fmt.Sprintf(mkdirURL, rrffm.baseURL, url.QueryEscape(path)))
	return rrffm.checkError(fmt.Sprintf("Mkdir %s", path), resp, err)
}

func (rrffm *rrffm) Move(oldpath, newpath string) error {
	resp, _, err := rrffm.doGetRequest(fmt.Sprintf(moveURL, rrffm.baseURL, url.QueryEscape(oldpath), url.QueryEscape(newpath)))
	return rrffm.checkError(fmt.Sprintf("Rename %s to %s", oldpath, newpath), resp, err)
}

func (rrffm *rrffm) MoveOverwrite(oldpath, newpath string) error {
	if err := rrffm.Delete(newpath); err != nil {
		return err
	}
	return rrffm.Move(oldpath, newpath)
}

func (rrffm *rrffm) Delete(path string) error {
	resp, _, err := rrffm.doGetRequest(fmt.Sprintf(deleteURL, rrffm.baseURL, url.QueryEscape(path)))
	return rrffm.checkError(fmt.Sprintf("Delete %s", path), resp, err)
}

func (rrffm *rrffm) DeleteRecursive(path string) error {
	fl, err := rrffm.GetFilelist(path)
	if err != nil {
		return err
	}
	for _, f := range fl.Files {
		if !f.IsDir() {

			// Directories come first so once we get here we can skip the remaining
			break
		}
		if err = rrffm.DeleteRecursive(fmt.Sprint("%s/%s", fl.Dir, f.Name)); err != nil {
			return err
		}
	}
	for _, f := range fl.Files {
		rrffm.Delete(f.Name)
	}
	return nil
}

func (rrffm *rrffm) Upload(path string, content []byte) (*time.Duration, error) {
	resp, duration, err := rrffm.doPostRequest(fmt.Sprintf(uploadURL, rrffm.baseURL, url.QueryEscape(path), url.QueryEscape(rrffm.getTimestamp(time.Now()))), content)
	return duration, rrffm.checkError(fmt.Sprintf("Uploading file to %s", path), resp, err)
}
