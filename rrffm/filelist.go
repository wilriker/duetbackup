package rrffm

import "time"

type localTime struct {
	Time time.Time
}

func (lt *localTime) UnmarshalJSON(b []byte) (err error) {
	// Parse date string in local time (it does not provide any timezone information)
	lt.Time, err = time.ParseInLocation(`"`+TimeFormat+`"`, string(b), time.Local)
	return err
}

// file resembles the JSON object returned in the files property of the rr_filelist response
type file struct {
	Type      string
	Name      string
	Size      uint64
	Timestamp localTime `json:"date"`
}

func (f *file) Date() time.Time {
	return f.Timestamp.Time
}

func (f *file) IsDir() bool {
	return f.Type == typeDirectory
}

func (f *file) IsFile() bool {
	return f.Type == typeFile
}

// Filelist resembled the JSON object in rr_filelist
type Filelist struct {
	Dir   string
	Files []file
	next  uint64
}
