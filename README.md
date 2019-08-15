# Hotsauce S3 Benchmark Version 0.1

## Introduction

hsbench is an S3 compatable benchmark originally based on [wasabi-tech/s3-benchmark](https://github.com/wasabi-tech/s3-benchmark).
While there are already several S3 compatable benchmark suites available, each has various tradeoffs.  What seemed to be missing
was a simple yet extremely fast benchmark that could easily be scripted into orchestration frameworks to run tests at scale.
hsbench tries to fill that niche.  The current release of hsbench is considered to be alpha level software and may contain bugs.

## Features

hsbench tries to improve on the original Wasabi s3-benchmark in the following ways:
*	Threads can distribute IOS across an arbitrary number of buckets.
*	Tests can be run individually and externally coordinated across multiple clients.
*	Intermediate results are logged periodically at user-defined intervals.
*	Min/avg/max/percentile latency results are included.
*	Test length can be limited either by duration or maximum number of objects.
*	Object prefixes can be set to test large object names (12 bytes reserved for uniqueness)
*	Bucket/Object prefixes can be used to allow multiple clients to target the same buckets

## Limitations

*	hsbench does not yet log log/csv/json output (will be added pending community feedback)
*	hsbench has no built-in provisions for making graphs
*	hsbench does not yet support mixed IO workloads
*	hsbench is still in alpha and options/output may change at any moment

## Prerequisites
To leverage this tool, the following prerequisites apply:
*	Git development environment
*	Go version 1.7 or newer
*	Access to an S3 compatable storage service (AWS, Ceph, etc) 

## Install

The easiest way to install hsbench is to use go's built in github support:

```
go get github.com/markhpc/hsbench
```

Then in the hsbench src directory run 'go build':

```
$ pwd
/home/perf/go/src/github.com/markhpc/hsbench
$ go build
```

## Usage

```
$ ./hsbench --help

USAGE: ./hsbench [OPTIONS]

OPTIONS:
  -a string
    	Access key
  -b int
    	Number of buckets to distribute IOs across (default 1)
  -bp string
    	Prefix for buckets (default "hotsauce_bench")
  -d int
    	Maximum test duration in seconds <-1 for unlimited> (default 60)
  -l int
    	Number of times to repeat test (default 1)
  -m string
    	Run modes in order.  See NOTES for more info (default "cxipgdx")
  -n int
    	Maximum number of objects <-1 for unlimited> (default -1)
  -op string
    	Prefix for objects
  -r string
    	Region for testing (default "us-east-1")
  -ri float
    	Number of seconds between report intervals (default 1)
  -s string
    	Secret key
  -t int
    	Number of threads to run (default 1)
  -u string
    	URL for host with method prefix
  -z string
    	Size of objects in bytes with postfix K, M, and G (default "1M")

NOTES:
  - Valid mode types for the -m mode string are:
    c: clear all existing objects from buckets (requires lookups)
    x: delete buckets
    i: initialize buckets 
    p: put objects in buckets
    g: get objects from buckets
    d: delete objects from buckets 

    These modes are processed in-order and can be repeated, ie "ippgd" will
    initialize the buckets, put the objects, reput the objects, get the
    objects, and then delete the objects.  The repeat flag will repeat this
    whole process the specified number of times.
```

## Example Benchmark

Below is an example run of the benchmark using a 10s test duration, 10 threads, 
10 buckets, and a 4K object size against a Ceph RadosGW backed by a single Ceph OSD 
running on an Intel P3700 NVMe device.  

```
$ ./hsbench -a <access key> -s <secret key> -u http://127.0.0.1:7480 -z 4K -d 10 -t 10 -b 10
2019/08/15 18:32:36 Hotsauce S3 Benchmark Version 0.1
2019/08/15 18:32:36 Parameters:
2019/08/15 18:32:36 url=http://127.0.0.1:7480
2019/08/15 18:32:36 object_prefix=
2019/08/15 18:32:36 bucket_prefix=hotsauce_bench
2019/08/15 18:32:36 region=us-east-1
2019/08/15 18:32:36 modes=cxipgdx
2019/08/15 18:32:36 object_count=-1
2019/08/15 18:32:36 bucket_count=10
2019/08/15 18:32:36 duration=10
2019/08/15 18:32:36 threads=10
2019/08/15 18:32:36 loops=1
2019/08/15 18:32:36 size=4K
2019/08/15 18:32:36 interval=1.000000
2019/08/15 18:32:36 Running Loop 0 BUCKET CLEAR TEST
2019/08/15 18:32:36 Loop: 0, Int: ALL, Dur(s): 0.0, Mode: BCLR, Ops: 0, MB/s: 0.00, IO/s: 0, Lat(ms): [ min: 0.0, avg: 0.0, 99%: 0.0, max: 0.0 ], Slowdowns: 0
2019/08/15 18:32:36 Running Loop 0 BUCKET DELETE TEST
2019/08/15 18:32:36 Loop: 0, Int: ALL, Dur(s): 0.0, Mode: BDEL, Ops: 0, MB/s: 0.00, IO/s: 0, Lat(ms): [ min: 0.0, avg: 0.0, 99%: 0.0, max: 0.0 ], Slowdowns: 0
2019/08/15 18:32:36 Running Loop 0 BUCKET INIT TEST
2019/08/15 18:32:36 Loop: 0, Int: ALL, Dur(s): 0.0, Mode: BINIT, Ops: 10, MB/s: 0.00, IO/s: 796, Lat(ms): [ min: 10.1, avg: 11.3, 99%: 12.4, max: 12.4 ], Slowdowns: 0
2019/08/15 18:32:36 Running Loop 0 OBJECT PUT TEST
2019/08/15 18:32:37 Loop: 0, Int: 1, Dur(s): 1.0, Mode: PUT, Ops: 5201, MB/s: 20.32, IO/s: 5201, Lat(ms): [ min: 1.2, avg: 1.9, 99%: 3.7, max: 6.1 ], Slowdowns: 0
2019/08/15 18:32:38 Loop: 0, Int: 2, Dur(s): 1.0, Mode: PUT, Ops: 5372, MB/s: 20.98, IO/s: 5372, Lat(ms): [ min: 1.3, avg: 1.9, 99%: 3.3, max: 4.3 ], Slowdowns: 0
2019/08/15 18:32:39 Loop: 0, Int: 3, Dur(s): 1.0, Mode: PUT, Ops: 5270, MB/s: 20.59, IO/s: 5270, Lat(ms): [ min: 1.2, avg: 1.9, 99%: 3.4, max: 10.3 ], Slowdowns: 0
2019/08/15 18:32:40 Loop: 0, Int: 4, Dur(s): 1.0, Mode: PUT, Ops: 5280, MB/s: 20.62, IO/s: 5280, Lat(ms): [ min: 1.2, avg: 1.9, 99%: 3.4, max: 9.4 ], Slowdowns: 0
2019/08/15 18:32:41 Loop: 0, Int: 5, Dur(s): 1.0, Mode: PUT, Ops: 5356, MB/s: 20.92, IO/s: 5356, Lat(ms): [ min: 1.3, avg: 1.9, 99%: 3.2, max: 4.3 ], Slowdowns: 0
2019/08/15 18:32:42 Loop: 0, Int: 6, Dur(s): 1.0, Mode: PUT, Ops: 5279, MB/s: 20.62, IO/s: 5279, Lat(ms): [ min: 1.3, avg: 1.9, 99%: 3.0, max: 11.8 ], Slowdowns: 0
2019/08/15 18:32:43 Loop: 0, Int: 7, Dur(s): 1.0, Mode: PUT, Ops: 5296, MB/s: 20.69, IO/s: 5296, Lat(ms): [ min: 1.2, avg: 1.9, 99%: 3.0, max: 10.2 ], Slowdowns: 0
2019/08/15 18:32:44 Loop: 0, Int: 8, Dur(s): 1.0, Mode: PUT, Ops: 5319, MB/s: 20.78, IO/s: 5319, Lat(ms): [ min: 1.3, avg: 1.9, 99%: 2.9, max: 3.7 ], Slowdowns: 0
2019/08/15 18:32:45 Loop: 0, Int: 9, Dur(s): 1.0, Mode: PUT, Ops: 5161, MB/s: 20.16, IO/s: 5161, Lat(ms): [ min: 1.2, avg: 1.9, 99%: 3.1, max: 9.8 ], Slowdowns: 0
2019/08/15 18:32:46 Loop: 0, Int: 10, Dur(s): 1.0, Mode: PUT, Ops: 4768, MB/s: 18.62, IO/s: 4768, Lat(ms): [ min: 1.2, avg: 2.1, 99%: 3.0, max: 89.3 ], Slowdowns: 0
2019/08/15 18:32:46 Loop: 0, Int: ALL, Dur(s): 10.0, Mode: PUT, Ops: 52312, MB/s: 20.43, IO/s: 5230, Lat(ms): [ min: 1.2, avg: 1.9, 99%: 3.2, max: 89.3 ], Slowdowns: 0
2019/08/15 18:32:46 Running Loop 0 OBJECT GET TEST
2019/08/15 18:32:47 Loop: 0, Int: 1, Dur(s): 1.0, Mode: GET, Ops: 1190, MB/s: 4.65, IO/s: 1190, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.3, max: 1.6 ], Slowdowns: 0
2019/08/15 18:32:48 Loop: 0, Int: 2, Dur(s): 1.0, Mode: GET, Ops: 1142, MB/s: 4.46, IO/s: 1142, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.1, max: 1.7 ], Slowdowns: 0
2019/08/15 18:32:49 Loop: 0, Int: 3, Dur(s): 1.0, Mode: GET, Ops: 1111, MB/s: 4.34, IO/s: 1111, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.1, max: 1.2 ], Slowdowns: 0
2019/08/15 18:32:50 Loop: 0, Int: 4, Dur(s): 1.0, Mode: GET, Ops: 1113, MB/s: 4.35, IO/s: 1113, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.1, max: 3.7 ], Slowdowns: 0
2019/08/15 18:32:51 Loop: 0, Int: 5, Dur(s): 1.0, Mode: GET, Ops: 1083, MB/s: 4.23, IO/s: 1083, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.2, max: 1.4 ], Slowdowns: 0
2019/08/15 18:32:52 Loop: 0, Int: 6, Dur(s): 1.0, Mode: GET, Ops: 1089, MB/s: 4.25, IO/s: 1089, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.1, max: 1.4 ], Slowdowns: 0
2019/08/15 18:32:53 Loop: 0, Int: 7, Dur(s): 1.0, Mode: GET, Ops: 1122, MB/s: 4.38, IO/s: 1122, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.2, max: 1.5 ], Slowdowns: 0
2019/08/15 18:32:54 Loop: 0, Int: 8, Dur(s): 1.0, Mode: GET, Ops: 1089, MB/s: 4.25, IO/s: 1089, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.1, max: 1.4 ], Slowdowns: 0
2019/08/15 18:32:55 Loop: 0, Int: 9, Dur(s): 1.0, Mode: GET, Ops: 1059, MB/s: 4.14, IO/s: 1059, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.1, max: 5.7 ], Slowdowns: 0
2019/08/15 18:32:56 Loop: 0, Int: ALL, Dur(s): 10.0, Mode: GET, Ops: 11086, MB/s: 4.31, IO/s: 1104, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.1, max: 5.7 ], Slowdowns: 0
2019/08/15 18:32:56 Running Loop 0 OBJECT DELETE TEST
2019/08/15 18:32:57 Loop: 0, Int: 1, Dur(s): 1.0, Mode: DEL, Ops: 5716, MB/s: 22.33, IO/s: 5716, Lat(ms): [ min: 1.1, avg: 1.7, 99%: 2.7, max: 3.8 ], Slowdowns: 0
2019/08/15 18:32:58 Loop: 0, Int: 2, Dur(s): 1.0, Mode: DEL, Ops: 5661, MB/s: 22.11, IO/s: 5661, Lat(ms): [ min: 1.1, avg: 1.8, 99%: 2.8, max: 9.6 ], Slowdowns: 0
2019/08/15 18:32:59 Loop: 0, Int: 3, Dur(s): 1.0, Mode: DEL, Ops: 5616, MB/s: 21.94, IO/s: 5616, Lat(ms): [ min: 1.2, avg: 1.8, 99%: 2.9, max: 10.1 ], Slowdowns: 0
2019/08/15 18:33:00 Loop: 0, Int: 4, Dur(s): 1.0, Mode: DEL, Ops: 5608, MB/s: 21.91, IO/s: 5608, Lat(ms): [ min: 1.2, avg: 1.8, 99%: 2.7, max: 7.7 ], Slowdowns: 0
2019/08/15 18:33:01 Loop: 0, Int: 5, Dur(s): 1.0, Mode: DEL, Ops: 5529, MB/s: 21.60, IO/s: 5529, Lat(ms): [ min: 1.2, avg: 1.8, 99%: 3.0, max: 9.3 ], Slowdowns: 0
2019/08/15 18:33:02 Loop: 0, Int: 6, Dur(s): 1.0, Mode: DEL, Ops: 5438, MB/s: 21.24, IO/s: 5438, Lat(ms): [ min: 1.2, avg: 1.8, 99%: 3.0, max: 10.4 ], Slowdowns: 0
2019/08/15 18:33:03 Loop: 0, Int: 7, Dur(s): 1.0, Mode: DEL, Ops: 5515, MB/s: 21.54, IO/s: 5515, Lat(ms): [ min: 1.3, avg: 1.8, 99%: 2.9, max: 7.5 ], Slowdowns: 0
2019/08/15 18:33:04 Loop: 0, Int: 8, Dur(s): 1.0, Mode: DEL, Ops: 5481, MB/s: 21.41, IO/s: 5481, Lat(ms): [ min: 1.2, avg: 1.8, 99%: 3.0, max: 9.7 ], Slowdowns: 0
2019/08/15 18:33:05 Loop: 0, Int: 9, Dur(s): 1.0, Mode: DEL, Ops: 5614, MB/s: 21.93, IO/s: 5614, Lat(ms): [ min: 1.2, avg: 1.8, 99%: 3.1, max: 8.0 ], Slowdowns: 0
2019/08/15 18:33:05 Loop: 0, Int: ALL, Dur(s): 9.4, Mode: DEL, Ops: 52312, MB/s: 21.76, IO/s: 5571, Lat(ms): [ min: 1.1, avg: 1.8, 99%: 2.9, max: 10.4 ], Slowdowns: 0
2019/08/15 18:33:05 Running Loop 0 BUCKET DELETE TEST
2019/08/15 18:33:05 Loop: 0, Int: ALL, Dur(s): 0.1, Mode: BDEL, Ops: 10, MB/s: 0.00, IO/s: 133, Lat(ms): [ min: 54.1, avg: 68.9, 99%: 75.2, max: 75.2 ], Slowdowns: 0
```

