package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	sysDir = "0:/sys"
)

var offsetString string
var once sync.Once

type file struct {
	Type string
	Name string
	Size uint64
	Date time.Time `json:"date,string"`
}

type filelist struct {
	Dir   string
	Files []file
}

func (f *file) UnmarshalJSON(b []byte) (err error) {
	var raw map[string]interface{}

	// Let regular Unmarshal do the main work
	err = json.Unmarshal(b, &raw)
	if err != nil {
		return err
	}

	for k, v := range raw {
		switch k {
		case "type":
			f.Type = v.(string)
		case "name":
			f.Name = v.(string)
		case "size":
			f.Size = uint64(v.(float64))
		case "date":
			// FIXME This needs to be solved better!
			// Get timezone offset to append to the date string that has no timezone offset
			once.Do(func() {
				loc, _ := time.LoadLocation("Europe/Berlin")
				_, offset := time.Now().In(loc).Zone()
				o := int64(offset) / 3600
				offsetString = fmt.Sprintf("%+03d:00", o)
			})

			// Parse date string
			d, err := time.Parse(time.RFC3339, v.(string)+offsetString)
			if err != nil {
				return err
			}
			f.Date = d
		}
	}
	return nil
}

func getFileList(baseURL string, dir string) (*filelist, error) {

	fileListURL := "rr_filelist?dir="

	resp, err := http.Get(baseURL + fileListURL + dir)
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
	return &fl, nil
}

func updateLocalFiles(baseURL string, fl *filelist, outDir string) error {

	fileDownloadURL := "rr_download?name=" + html.EscapeString(fl.Dir) + "/"

	for _, file := range fl.Files {
		fileName := outDir + "/" + file.Name
		fi, err := os.Stat(fileName)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		if fi == nil && file.Type == "d" {
			fmt.Println("Adding new directory", file.Name)
			err = os.MkdirAll(fileName, 0755)
			if err != nil {
				return err
			}
			continue
		}

		// File does not exist or is outdated so get it
		if fi == nil || fi.ModTime().Before(file.Date) {

			// Download file
			resp, err := http.Get(baseURL + fileDownloadURL + html.EscapeString(file.Name))
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
			if fi != nil {
				fmt.Println("Updating file", file.Name)
			} else {
				fmt.Println("Adding new file", file.Name)
			}

			// Write contents to local file
			_, err = nf.Write(body)
			if err != nil {
				return err
			}

			// Adjust mtime
			os.Chtimes(fileName, file.Date, file.Date)
		} else {
			fmt.Println(file.Name, "is up-to-date")
		}

	}

	return nil
}

func getAddress(domain string, port uint64) string {
	return "http://" + domain + ":" + strconv.FormatUint(port, 10) + "/"
}

func connect(address, password string) error {
	path := "rr_connect?password=" + html.EscapeString(password) + "&time=" + html.EscapeString(time.Now().Format("2006-01-02T15:04:05"))
	_, err := http.Get(address + path)
	return err
}

func main() {
	var domain, dirToBackup, outDir, password string
	var port uint64

	flag.StringVar(&domain, "domain", "", "Domain of Duet Wifi")
	flag.Uint64Var(&port, "port", 80, "Port of Duet Wifi")
	flag.StringVar(&dirToBackup, "dirToBackup", sysDir, "Directory on Duet to create a backup of")
	flag.StringVar(&outDir, "outDir", "", "Output dir of backup")
	flag.StringVar(&password, "password", "reprap", "Connection password")
	flag.Parse()

	if domain == "" || outDir == "" {
		fmt.Println("domain and outDir are required")
		os.Exit(1)
	}

	address := getAddress(domain, port)

	// Try to connect
	if err := connect(address, password); err != nil {
		fmt.Println("Duet currently not available")
		os.Exit(0)
	}

	fl, err := getFileList(address, html.EscapeString(dirToBackup))
	if err != nil {
		panic(err)
	}

	err = updateLocalFiles(address, fl, outDir)
	if err != nil {
		panic(err)
	}
}
