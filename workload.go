package main

import (
	"gopkg.in/yaml.v2"
	"os"
)

type WorkloadProfileEntry struct {
	Bucket string `yaml:"bucket"`
	Count  int64  `yaml:"count"`
	Size   int64  `yaml:"size"`
	Offset int64  `yaml:"offset"`
}

type WorkloadProfile struct {
	Name    string                 `yaml:"name"`
	Entries []WorkloadProfileEntry `yaml:"entries"`
	len     int64
}

type WorkloadConfig struct {
	Profiles []WorkloadProfile `yaml:"profiles"`
}

func LoadWorkloadConfig(fn string) (p WorkloadConfig, err error) {
	f, err := os.ReadFile(fn)
	if err != nil {
		return
	}
	err = yaml.Unmarshal(f, &p)
	if err == nil {
		p.Init()
	}
	return
}

func (W *WorkloadConfig) Init() {
	for id, e := range W.Profiles {
		var l int64
		for _, x := range e.Entries {
			l += x.Count
		}

		// Basic init if no entries OR len < 1
		if (len(e.Entries) < 1) || (l < 1) {
			e.Entries = make([]WorkloadProfileEntry, 0)
			e.Entries = append(e.Entries, WorkloadProfileEntry{Bucket: "", Count: 1, Size: 0, Offset: 0})
			l = 1
		}
		W.Profiles[id].len = l
	}
	return
}

func (W *WorkloadConfig) AddDefaultProfile(bucket string, count int64, size int64, offset int64) {
	wp := WorkloadProfile{
		Name:    "",
		Entries: make([]WorkloadProfileEntry, 0),
		len:     0,
	}
	wp.Entries = append(wp.Entries, WorkloadProfileEntry{Bucket: bucket, Count: count, Size: size, Offset: offset})
	W.Profiles = append(W.Profiles, wp)
	W.Init()
}

// Get entry for specified iteration
func (W *WorkloadConfig) GetEntry(profileId int, iter int64) WorkloadProfileEntry {
	p := W.Profiles[profileId]
	iter = iter % p.len
	for _, e := range p.Entries {
		iter -= e.Count
		if iter <= 0 {
			return e
		}
	}
	return p.Entries[0]
}
