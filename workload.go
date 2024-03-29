package main

import (
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
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

type S3ConfigEntry struct {
	Name      string   `yaml:"name"`
	Endpoints []string `yaml:"endpoints"`
	AccessKey string   `yaml:"access_key"`
	SecretKey string   `yaml:"secret_key"`
}

type WorkloadConfig struct {
	Profiles []WorkloadProfile `yaml:"profiles"`
	S3Config []S3ConfigEntry   `yaml:"s3"`
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

func (W *WorkloadConfig) AddS3Config(name string, endpoints []string, access_key string, secret_key string) {
	s3 := S3ConfigEntry{
		Name:      name,
		Endpoints: endpoints,
		AccessKey: access_key,
		SecretKey: secret_key,
	}
	W.S3Config = append(W.S3Config, s3)
	return
}

func (W *WorkloadConfig) AddWorkloadProfile(bucket string, count int64, size int64, offset int64) {
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

func GetS3Services(profile string) []*s3.S3 {
	s3Config := workload_config.S3Config[0]
	s3Endpoints := s3Config.Endpoints

	//svc := s3.New(session.New(), cfg)
	svcL := make([]*s3.S3, len(s3Endpoints))
	for i, s := range s3Endpoints {
		c := cfg
		s0 := s
		c.Endpoint = &s0
		c.Credentials = credentials.NewStaticCredentials(s3Config.AccessKey, s3Config.SecretKey, "")
		svcL[i] = s3.New(session.New(), c)
	}
	return svcL
}
