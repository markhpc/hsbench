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
  -o string
    	Write CSV output to this file
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
$ ./hsbench -a 3JZ0SVK94Z55OZU5J1N0 -s OdzEPyDDZ0ls1haDUu1NVWkJDcnG74Lb7XylfXRM -u http://127.0.0.1:7480 -z 4K -d 10 -t 10 -b 10 -o test.csv
2019/08/19 08:18:51 Hotsauce S3 Benchmark Version 0.1
2019/08/19 08:18:51 Parameters:
2019/08/19 08:18:51 url=http://127.0.0.1:7480
2019/08/19 08:18:51 object_prefix=
2019/08/19 08:18:51 bucket_prefix=hotsauce_bench
2019/08/19 08:18:51 region=us-east-1
2019/08/19 08:18:51 modes=cxipgdx
2019/08/19 08:18:51 object_count=-1
2019/08/19 08:18:51 bucket_count=10
2019/08/19 08:18:51 duration=10
2019/08/19 08:18:51 threads=10
2019/08/19 08:18:51 loops=1
2019/08/19 08:18:51 size=4K
2019/08/19 08:18:51 interval=1.000000
2019/08/19 08:18:51 Running Loop 0 BUCKET CLEAR TEST
2019/08/19 08:18:51 Loop: 0, Int: TOTAL, Dur(s): 0.0, Mode: BCLR, Ops: 0, MB/s: 0.00, IO/s: 0, Lat(ms): [ min: 0.0, avg: 0.0, 99%: 0.0, max: 0.0 ], Slowdowns: 0
2019/08/19 08:18:51 Running Loop 0 BUCKET DELETE TEST
2019/08/19 08:18:51 Loop: 0, Int: TOTAL, Dur(s): 0.0, Mode: BDEL, Ops: 0, MB/s: 0.00, IO/s: 0, Lat(ms): [ min: 0.0, avg: 0.0, 99%: 0.0, max: 0.0 ], Slowdowns: 0
2019/08/19 08:18:51 Running Loop 0 BUCKET INIT TEST
2019/08/19 08:18:51 Loop: 0, Int: TOTAL, Dur(s): 0.0, Mode: BINIT, Ops: 10, MB/s: 0.00, IO/s: 968, Lat(ms): [ min: 7.5, avg: 8.7, 99%: 10.2, max: 10.2 ], Slowdowns: 0
2019/08/19 08:18:51 Running Loop 0 OBJECT PUT TEST
2019/08/19 08:18:52 Loop: 0, Int: 0, Dur(s): 1.0, Mode: PUT, Ops: 5255, MB/s: 20.53, IO/s: 5255, Lat(ms): [ min: 1.3, avg: 1.9, 99%: 3.4, max: 14.1 ], Slowdowns: 0
2019/08/19 08:18:53 Loop: 0, Int: 1, Dur(s): 1.0, Mode: PUT, Ops: 5237, MB/s: 20.46, IO/s: 5237, Lat(ms): [ min: 1.2, avg: 1.9, 99%: 3.4, max: 11.1 ], Slowdowns: 0
2019/08/19 08:18:54 Loop: 0, Int: 2, Dur(s): 1.0, Mode: PUT, Ops: 5454, MB/s: 21.30, IO/s: 5454, Lat(ms): [ min: 1.2, avg: 1.8, 99%: 2.9, max: 7.3 ], Slowdowns: 0
2019/08/19 08:18:55 Loop: 0, Int: 3, Dur(s): 1.0, Mode: PUT, Ops: 5318, MB/s: 20.77, IO/s: 5318, Lat(ms): [ min: 1.3, avg: 1.9, 99%: 3.7, max: 7.1 ], Slowdowns: 0
2019/08/19 08:18:56 Loop: 0, Int: 4, Dur(s): 1.0, Mode: PUT, Ops: 5364, MB/s: 20.95, IO/s: 5364, Lat(ms): [ min: 1.3, avg: 1.9, 99%: 2.9, max: 7.8 ], Slowdowns: 0
2019/08/19 08:18:57 Loop: 0, Int: 5, Dur(s): 1.0, Mode: PUT, Ops: 5219, MB/s: 20.39, IO/s: 5219, Lat(ms): [ min: 1.3, avg: 1.9, 99%: 3.2, max: 16.8 ], Slowdowns: 0
2019/08/19 08:18:58 Loop: 0, Int: 6, Dur(s): 1.0, Mode: PUT, Ops: 5200, MB/s: 20.31, IO/s: 5200, Lat(ms): [ min: 1.2, avg: 1.9, 99%: 3.1, max: 9.6 ], Slowdowns: 0
2019/08/19 08:18:59 Loop: 0, Int: 7, Dur(s): 1.0, Mode: PUT, Ops: 5250, MB/s: 20.51, IO/s: 5250, Lat(ms): [ min: 1.3, avg: 1.9, 99%: 3.0, max: 7.3 ], Slowdowns: 0
2019/08/19 08:19:00 Loop: 0, Int: 8, Dur(s): 1.0, Mode: PUT, Ops: 4701, MB/s: 18.36, IO/s: 4701, Lat(ms): [ min: 1.3, avg: 2.1, 99%: 3.5, max: 86.6 ], Slowdowns: 0
2019/08/19 08:19:01 Loop: 0, Int: 9, Dur(s): 1.0, Mode: PUT, Ops: 5269, MB/s: 20.58, IO/s: 5269, Lat(ms): [ min: 1.3, avg: 1.9, 99%: 3.2, max: 11.4 ], Slowdowns: 0
2019/08/19 08:19:01 Loop: 0, Int: TOTAL, Dur(s): 10.0, Mode: PUT, Ops: 52277, MB/s: 20.42, IO/s: 5227, Lat(ms): [ min: 1.2, avg: 1.9, 99%: 3.2, max: 86.6 ], Slowdowns: 0
2019/08/19 08:19:01 Running Loop 0 OBJECT GET TEST
2019/08/19 08:19:03 Loop: 0, Int: 0, Dur(s): 1.0, Mode: GET, Ops: 1275, MB/s: 4.98, IO/s: 1275, Lat(ms): [ min: 0.6, avg: 0.9, 99%: 1.4, max: 1.5 ], Slowdowns: 0
2019/08/19 08:19:04 Loop: 0, Int: 1, Dur(s): 1.0, Mode: GET, Ops: 1286, MB/s: 5.02, IO/s: 1286, Lat(ms): [ min: 0.6, avg: 0.9, 99%: 1.3, max: 1.8 ], Slowdowns: 0
2019/08/19 08:19:05 Loop: 0, Int: 2, Dur(s): 1.0, Mode: GET, Ops: 1112, MB/s: 4.34, IO/s: 1112, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.1, max: 1.6 ], Slowdowns: 0
2019/08/19 08:19:06 Loop: 0, Int: 3, Dur(s): 1.0, Mode: GET, Ops: 1128, MB/s: 4.41, IO/s: 1128, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.2, max: 5.7 ], Slowdowns: 0
2019/08/19 08:19:07 Loop: 0, Int: 4, Dur(s): 1.0, Mode: GET, Ops: 1112, MB/s: 4.34, IO/s: 1112, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.2, max: 1.7 ], Slowdowns: 0
2019/08/19 08:19:08 Loop: 0, Int: 5, Dur(s): 1.0, Mode: GET, Ops: 1174, MB/s: 4.59, IO/s: 1174, Lat(ms): [ min: 0.6, avg: 0.9, 99%: 1.3, max: 1.6 ], Slowdowns: 0
2019/08/19 08:19:09 Loop: 0, Int: 6, Dur(s): 1.0, Mode: GET, Ops: 1106, MB/s: 4.32, IO/s: 1106, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.1, max: 1.7 ], Slowdowns: 0
2019/08/19 08:19:10 Loop: 0, Int: 7, Dur(s): 1.0, Mode: GET, Ops: 1132, MB/s: 4.42, IO/s: 1132, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.2, max: 11.6 ], Slowdowns: 0
2019/08/19 08:19:11 Loop: 0, Int: 8, Dur(s): 1.0, Mode: GET, Ops: 1074, MB/s: 4.20, IO/s: 1074, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.2, max: 1.4 ], Slowdowns: 0
2019/08/19 08:19:12 Loop: 0, Int: 9, Dur(s): 1.0, Mode: GET, Ops: 1080, MB/s: 4.22, IO/s: 1080, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.2, max: 1.6 ], Slowdowns: 0
2019/08/19 08:19:12 Loop: 0, Int: TOTAL, Dur(s): 10.0, Mode: GET, Ops: 11481, MB/s: 4.47, IO/s: 1144, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.2, max: 11.6 ], Slowdowns: 0
2019/08/19 08:19:12 Running Loop 0 OBJECT DELETE TEST
2019/08/19 08:19:13 Loop: 0, Int: 0, Dur(s): 1.0, Mode: DEL, Ops: 5591, MB/s: 21.84, IO/s: 5591, Lat(ms): [ min: 1.2, avg: 1.8, 99%: 3.3, max: 11.0 ], Slowdowns: 0
2019/08/19 08:19:14 Loop: 0, Int: 1, Dur(s): 1.0, Mode: DEL, Ops: 5607, MB/s: 21.90, IO/s: 5607, Lat(ms): [ min: 1.2, avg: 1.8, 99%: 3.1, max: 13.9 ], Slowdowns: 0
2019/08/19 08:19:15 Loop: 0, Int: 2, Dur(s): 1.0, Mode: DEL, Ops: 5680, MB/s: 22.19, IO/s: 5680, Lat(ms): [ min: 1.2, avg: 1.8, 99%: 2.7, max: 7.4 ], Slowdowns: 0
2019/08/19 08:19:16 Loop: 0, Int: 3, Dur(s): 1.0, Mode: DEL, Ops: 5691, MB/s: 22.23, IO/s: 5691, Lat(ms): [ min: 1.2, avg: 1.8, 99%: 2.9, max: 6.5 ], Slowdowns: 0
2019/08/19 08:19:17 Loop: 0, Int: 4, Dur(s): 1.0, Mode: DEL, Ops: 5565, MB/s: 21.74, IO/s: 5565, Lat(ms): [ min: 1.2, avg: 1.8, 99%: 3.6, max: 11.0 ], Slowdowns: 0
2019/08/19 08:19:18 Loop: 0, Int: 5, Dur(s): 1.0, Mode: DEL, Ops: 5739, MB/s: 22.42, IO/s: 5739, Lat(ms): [ min: 1.2, avg: 1.7, 99%: 2.7, max: 3.5 ], Slowdowns: 0
2019/08/19 08:19:19 Loop: 0, Int: 6, Dur(s): 1.0, Mode: DEL, Ops: 5613, MB/s: 21.93, IO/s: 5613, Lat(ms): [ min: 1.2, avg: 1.8, 99%: 2.9, max: 7.1 ], Slowdowns: 0
2019/08/19 08:19:20 Loop: 0, Int: 7, Dur(s): 1.0, Mode: DEL, Ops: 5591, MB/s: 21.84, IO/s: 5591, Lat(ms): [ min: 1.2, avg: 1.8, 99%: 2.9, max: 16.3 ], Slowdowns: 0
2019/08/19 08:19:21 Loop: 0, Int: 8, Dur(s): 1.0, Mode: DEL, Ops: 5784, MB/s: 22.59, IO/s: 5784, Lat(ms): [ min: 1.2, avg: 1.7, 99%: 2.9, max: 14.3 ], Slowdowns: 0
2019/08/19 08:19:21 Loop: 0, Int: TOTAL, Dur(s): 9.3, Mode: DEL, Ops: 52277, MB/s: 22.04, IO/s: 5643, Lat(ms): [ min: 1.1, avg: 1.8, 99%: 3.0, max: 16.3 ], Slowdowns: 0
2019/08/19 08:19:21 Running Loop 0 BUCKET DELETE TEST
2019/08/19 08:19:21 Loop: 0, Int: TOTAL, Dur(s): 0.1, Mode: BDEL, Ops: 10, MB/s: 0.00, IO/s: 126, Lat(ms): [ min: 71.3, avg: 74.7, 99%: 79.3, max: 79.3 ], Slowdowns: 0
```

One notable point is that like the s3-benchmark program it is based on, hsbench has relatively low CPU overhead compared to some other S3 benchmarks.  During the 4K PUT phase of the above test:

```
22628 root      20   0 6700416 5.308g  28072 S 413.3  8.4 676:28.86 ceph-osd
23329 root      20   0 1658700 253068  21320 S 333.3  0.4 530:06.76 radosgw
 9017 perf      20   0 2184580  33612   6568 S 173.3  0.1   0:05.25 hsbench
```
