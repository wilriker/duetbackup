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
  -exclude value
        Exclude paths starting with this string (can be passed multiple times)
  -outDir string
        Output dir of backup
  -password string
        Connection password (default "reprap")
  -port uint
        Port of Duet Wifi (default 80)
  -removeLocal
        Remove files locally that have been deleted on the Duet
  -verbose
        Output more details
```

## Feedback
Please provide any feedback either here in the Issues or send a pull request or go to [the Duet3D forum](https://forum.duet3d.com/topic/10709/duetbackup-cli-tool-to-backup-your-duet-sd-card).
