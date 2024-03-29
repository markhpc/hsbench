package main

import (
	"encoding/csv"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
)

type ObjectInfo struct {
	Bucket   string
	Key      string
	Created  uint64
	Size     int64
	Duration time.Duration
	Error    string `json:",omitempty"`
}

func LogObjectInfo(fn string, logCh <-chan ObjectInfo, wg *sync.WaitGroup) {
	defer wg.Done()

	if fn != "" {
		file, err := os.OpenFile(fn, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0777)
		if err != nil {
			log.Fatal("Could not open file to write objects info: ", err)
		}

		for object_info := range logCh {
			data, err := json.Marshal(object_info)
			if err != nil {
				log.Fatal("Error marshaling object info for key '", object_info.Key, "': ", err)
				continue
			}
			_, err = file.Write(data)
			if err != nil {
				log.Fatal("Error writing object info for key '", object_info.Key, "': ", err)
				log.Fatal("Abort writing")
				break
			}
			_, err = file.WriteString("\n")
			if err != nil {
				log.Fatal("Error writing eol for key '", object_info.Key, "': ", err)
				log.Fatal("Abort writing")
				break
			}
		}

		file.Sync()
		file.Close()
	} else {
		for _ = range logCh {
		}
	}
}

func LogCSV(fn string, oStats []OutputStats) {
	if fn != "" {
		file, err := os.OpenFile(fn, os.O_CREATE|os.O_WRONLY, 0777)
		defer file.Close()
		if err != nil {
			log.Fatal("Could not open CSV file for writing:", fn, err)
		} else {
			csvWriter := csv.NewWriter(file)
			for i, o := range oStats {
				if i == 0 {
					o.csv_header(csvWriter)
				}
				o.csv(csvWriter)
			}
			csvWriter.Flush()
		}
	}
}

func LogJSON(fn string, oStats []OutputStats) {
	if fn != "" {
		file, err := os.OpenFile(fn, os.O_CREATE|os.O_WRONLY, 0777)
		defer file.Close()
		if err != nil {
			log.Fatal("Could not open JSON file for writing.", fn, err)
		}
		data, err := json.Marshal(oStats)
		if err != nil {
			log.Fatal("Error marshaling JSON: ", err)
		}
		_, err = file.Write(data)
		if err != nil {
			log.Fatal("Error writing to JSON file: ", err)
		}
		file.Sync()
	}
}
