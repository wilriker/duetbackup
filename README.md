# duetbackup
Small tool to fetch all files in a folder on a Duet board.
By default it will backup 0:/sys folder.

## Usage
```
Usage of ./duetbackup:
    -dirToBackup string
          Directory on Duet to create a backup of (default "0:/sys")
    -domain string
        Domain of Duet Wifi
    -keepLocal
        Keep files locally that have been deleted on the Duet
    -outDir string
          Output dir of backup
    -password string
        Connection password (default "reprap")
    -port uint
          Port of Duet Wifi (default 80)
```
