package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/wilriker/duetbackup"
	"github.com/wilriker/librfm"
)

func main() {
	var domain, dirToBackup, outDir, password string
	var removeLocal, verbose bool
	var port uint64
	var excls duetbackup.Excludes

	flag.StringVar(&domain, "domain", "", "Domain of Duet Wifi")
	flag.Uint64Var(&port, "port", 80, "Port of Duet Wifi")
	flag.StringVar(&password, "password", "reprap", "Connection password")
	flag.BoolVar(&verbose, "verbose", false, "Output more details")
	flag.StringVar(&dirToBackup, "dirToBackup", duetbackup.SysDir, "Directory on Duet to create a backup of")
	flag.StringVar(&outDir, "outDir", "", "Output dir of backup")
	flag.BoolVar(&removeLocal, "removeLocal", false, "Remove files locally that have been deleted on the Duet")
	flag.Var(&excls, "exclude", "Exclude paths starting with this string (can be passed multiple times)")
	flag.Parse()

	if domain == "" || outDir == "" {
		log.Fatal("-domain and -outDir are mandatory parameters")
	}

	if port > 65535 {
		log.Fatal("Invalid port", port)
	}

	rfm := librfm.New(domain, port)

	// Try to connect
	if verbose {
		log.Println("Trying to connect to Duet")
	}
	if err := rfm.Connect(password); err != nil {
		log.Println("Duet currently not available")
		os.Exit(0)
	}

	// Get absolute path from user's input
	absPath, err := filepath.Abs(outDir)
	if err != nil {
		// Fall back to original user's input
		absPath = outDir
	}

	db := duetbackup.New(rfm, verbose)
	if err = db.SyncFolder(duetbackup.CleanPath(dirToBackup), absPath, excls, removeLocal); err != nil {
		log.Fatal(err)
	}
}
