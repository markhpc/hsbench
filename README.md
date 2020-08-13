# Hotsauce S3 Benchmark

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
*	You can use hsbench to delete buckets in parallel: `./hsbench -a ... -s ... -u http://... -m cx -t 32 -bl "bucket1 bucket2 ..."` (-t is the number of threads)

## Limitations

*	hsbench does not currently support multiple AWS endpoints
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
go get github.com/vitalif/hsbench
```

Then just use `~/go/bin/hsbench`.

If you want to patch and rebuild it, run `go build` or `go install` in the hsbench src directory:

```
$ cd ~/go/src/github.com/vitalif/hsbench
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
  -j string
    	Write JSON output to this file
  -l int
    	Number of times to repeat test (default 1)
  -m string
    	Run modes in order.  See NOTES for more info (default "cxiplgdcx")
  -mk int
    	Maximum number of keys to retreive at once for bucket listings (default 1000)
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
    l: list objects in buckets
    g: get objects from buckets
    d: delete objects from buckets 

    These modes are processed in-order and can be repeated, ie "ippgd" will
    initialize the buckets, put the objects, reput the objects, get the
    objects, and then delete the objects.  The repeat flag will repeat this
    whole process the specified number of times.

  - When performing bucket listings, many S3 storage systems limit the
    maximum number of keys returned to 1000 even if MaxKeys is set higher.
    hsbench will attempt to set MaxKeys to whatever value is passed via the 
    "mk" flag, but it's likely that any values above 1000 will be ignored.
```

## Example Benchmark

Below is an example run of the benchmark using a 10s test duration, 10 threads, 
10 buckets, and a 4K object size against a Ceph RadosGW backed by a single Ceph OSD 
running on an Intel P3700 NVMe device.  

```
$ ./hsbench -a 3JZ0SVK94Z55OZU5J1N0 -s OdzEPyDDZ0ls1haDUu1NVWkJDcnG74Lb7XylfXRM -u http://127.0.0.1:7480 -z 4K -d 10 -t 10 -b 10
2019/08/19 11:21:06 Hotsauce S3 Benchmark Version 0.1
2019/08/19 11:21:06 Parameters:
2019/08/19 11:21:06 url=http://127.0.0.1:7480
2019/08/19 11:21:06 object_prefix=
2019/08/19 11:21:06 bucket_prefix=hotsauce_bench
2019/08/19 11:21:06 region=us-east-1
2019/08/19 11:21:06 modes=cxipgdx
2019/08/19 11:21:06 output=
2019/08/19 11:21:06 json_output=
2019/08/19 11:21:06 object_count=-1
2019/08/19 11:21:06 bucket_count=10
2019/08/19 11:21:06 duration=10
2019/08/19 11:21:06 threads=10
2019/08/19 11:21:06 loops=1
2019/08/19 11:21:06 size=4K
2019/08/19 11:21:06 interval=1.000000
2019/08/19 11:21:06 Running Loop 0 BUCKET CLEAR TEST
2019/08/19 11:21:06 Loop: 0, Int: TOTAL, Dur(s): 0.0, Mode: BCLR, Ops: 0, MB/s: 0.00, IO/s: 0, Lat(ms): [ min: 0.0, avg: 0.0, 99%: 0.0, max: 0.0 ], Slowdowns: 0
2019/08/19 11:21:06 Running Loop 0 BUCKET DELETE TEST
2019/08/19 11:21:06 Loop: 0, Int: TOTAL, Dur(s): 0.0, Mode: BDEL, Ops: 0, MB/s: 0.00, IO/s: 0, Lat(ms): [ min: 0.0, avg: 0.0, 99%: 0.0, max: 0.0 ], Slowdowns: 0
2019/08/19 11:21:06 Running Loop 0 BUCKET INIT TEST
2019/08/19 11:21:06 Loop: 0, Int: TOTAL, Dur(s): 0.0, Mode: BINIT, Ops: 10, MB/s: 0.00, IO/s: 944, Lat(ms): [ min: 7.9, avg: 9.3, 99%: 10.4, max: 10.4 ], Slowdowns: 0
2019/08/19 11:21:06 Running Loop 0 OBJECT PUT TEST
2019/08/19 11:21:07 Loop: 0, Int: 0, Dur(s): 1.0, Mode: PUT, Ops: 5209, MB/s: 20.35, IO/s: 5209, Lat(ms): [ min: 1.3, avg: 1.9, 99%: 3.5, max: 8.4 ], Slowdowns: 0
2019/08/19 11:21:08 Loop: 0, Int: 1, Dur(s): 1.0, Mode: PUT, Ops: 5076, MB/s: 19.83, IO/s: 5076, Lat(ms): [ min: 1.2, avg: 2.0, 99%: 3.1, max: 42.3 ], Slowdowns: 0
2019/08/19 11:21:09 Loop: 0, Int: 2, Dur(s): 1.0, Mode: PUT, Ops: 4319, MB/s: 16.87, IO/s: 4319, Lat(ms): [ min: 1.3, avg: 2.3, 99%: 4.4, max: 58.0 ], Slowdowns: 0
2019/08/19 11:21:10 Loop: 0, Int: 3, Dur(s): 1.0, Mode: PUT, Ops: 4288, MB/s: 16.75, IO/s: 4288, Lat(ms): [ min: 1.3, avg: 2.3, 99%: 3.5, max: 63.1 ], Slowdowns: 0
2019/08/19 11:21:11 Loop: 0, Int: 4, Dur(s): 1.0, Mode: PUT, Ops: 4549, MB/s: 17.77, IO/s: 4549, Lat(ms): [ min: 1.3, avg: 2.2, 99%: 6.8, max: 57.9 ], Slowdowns: 0
2019/08/19 11:21:12 Loop: 0, Int: 5, Dur(s): 1.0, Mode: PUT, Ops: 4447, MB/s: 17.37, IO/s: 4447, Lat(ms): [ min: 1.3, avg: 2.2, 99%: 3.5, max: 58.8 ], Slowdowns: 0
2019/08/19 11:21:13 Loop: 0, Int: 6, Dur(s): 1.0, Mode: PUT, Ops: 4260, MB/s: 16.64, IO/s: 4260, Lat(ms): [ min: 1.3, avg: 2.4, 99%: 5.8, max: 58.4 ], Slowdowns: 0
2019/08/19 11:21:14 Loop: 0, Int: 7, Dur(s): 1.0, Mode: PUT, Ops: 5202, MB/s: 20.32, IO/s: 5202, Lat(ms): [ min: 1.3, avg: 1.9, 99%: 3.1, max: 10.3 ], Slowdowns: 0
2019/08/19 11:21:15 Loop: 0, Int: 8, Dur(s): 1.0, Mode: PUT, Ops: 5213, MB/s: 20.36, IO/s: 5213, Lat(ms): [ min: 1.3, avg: 1.9, 99%: 3.0, max: 13.3 ], Slowdowns: 0
2019/08/19 11:21:16 Loop: 0, Int: 9, Dur(s): 1.0, Mode: PUT, Ops: 5210, MB/s: 20.35, IO/s: 5210, Lat(ms): [ min: 1.2, avg: 1.9, 99%: 3.0, max: 9.6 ], Slowdowns: 0
2019/08/19 11:21:16 Loop: 0, Int: TOTAL, Dur(s): 10.0, Mode: PUT, Ops: 47783, MB/s: 18.66, IO/s: 4777, Lat(ms): [ min: 1.2, avg: 2.1, 99%: 3.4, max: 63.1 ], Slowdowns: 0
2019/08/19 11:21:16 Running Loop 0 OBJECT GET TEST
2019/08/19 11:21:17 Loop: 0, Int: 0, Dur(s): 1.0, Mode: GET, Ops: 1211, MB/s: 4.73, IO/s: 1211, Lat(ms): [ min: 0.6, avg: 0.9, 99%: 1.5, max: 2.0 ], Slowdowns: 0
2019/08/19 11:21:18 Loop: 0, Int: 1, Dur(s): 1.0, Mode: GET, Ops: 1182, MB/s: 4.62, IO/s: 1182, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.3, max: 2.1 ], Slowdowns: 0
2019/08/19 11:21:19 Loop: 0, Int: 2, Dur(s): 1.0, Mode: GET, Ops: 1110, MB/s: 4.34, IO/s: 1110, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.2, max: 1.5 ], Slowdowns: 0
2019/08/19 11:21:20 Loop: 0, Int: 3, Dur(s): 1.0, Mode: GET, Ops: 1072, MB/s: 4.19, IO/s: 1072, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.1, max: 1.3 ], Slowdowns: 0
2019/08/19 11:21:21 Loop: 0, Int: 4, Dur(s): 1.0, Mode: GET, Ops: 1098, MB/s: 4.29, IO/s: 1098, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.2, max: 3.9 ], Slowdowns: 0
2019/08/19 11:21:22 Loop: 0, Int: 5, Dur(s): 1.0, Mode: GET, Ops: 1115, MB/s: 4.36, IO/s: 1115, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.1, max: 1.4 ], Slowdowns: 0
2019/08/19 11:21:23 Loop: 0, Int: 6, Dur(s): 1.0, Mode: GET, Ops: 1110, MB/s: 4.34, IO/s: 1110, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.1, max: 1.5 ], Slowdowns: 0
2019/08/19 11:21:24 Loop: 0, Int: 7, Dur(s): 1.0, Mode: GET, Ops: 1079, MB/s: 4.21, IO/s: 1079, Lat(ms): [ min: 0.6, avg: 0.7, 99%: 1.1, max: 1.7 ], Slowdowns: 0
2019/08/19 11:21:25 Loop: 0, Int: 8, Dur(s): 1.0, Mode: GET, Ops: 1089, MB/s: 4.25, IO/s: 1089, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.2, max: 1.6 ], Slowdowns: 0
2019/08/19 11:21:26 Loop: 0, Int: 9, Dur(s): 1.0, Mode: GET, Ops: 1156, MB/s: 4.52, IO/s: 1156, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.2, max: 5.4 ], Slowdowns: 0
2019/08/19 11:21:26 Loop: 0, Int: TOTAL, Dur(s): 10.0, Mode: GET, Ops: 11222, MB/s: 4.37, IO/s: 1118, Lat(ms): [ min: 0.6, avg: 0.8, 99%: 1.2, max: 5.4 ], Slowdowns: 0
2019/08/19 11:21:26 Running Loop 0 OBJECT DELETE TEST
2019/08/19 11:21:27 Loop: 0, Int: 0, Dur(s): 1.0, Mode: DEL, Ops: 5673, MB/s: 22.16, IO/s: 5673, Lat(ms): [ min: 1.2, avg: 1.8, 99%: 2.7, max: 10.0 ], Slowdowns: 0
2019/08/19 11:21:28 Loop: 0, Int: 1, Dur(s): 1.0, Mode: DEL, Ops: 5597, MB/s: 21.86, IO/s: 5597, Lat(ms): [ min: 1.1, avg: 1.8, 99%: 2.9, max: 10.5 ], Slowdowns: 0
2019/08/19 11:21:29 Loop: 0, Int: 2, Dur(s): 1.0, Mode: DEL, Ops: 5123, MB/s: 20.01, IO/s: 5123, Lat(ms): [ min: 1.1, avg: 1.9, 99%: 3.2, max: 67.1 ], Slowdowns: 0
2019/08/19 11:21:30 Loop: 0, Int: 3, Dur(s): 1.0, Mode: DEL, Ops: 5547, MB/s: 21.67, IO/s: 5547, Lat(ms): [ min: 1.2, avg: 1.8, 99%: 2.7, max: 15.2 ], Slowdowns: 0
2019/08/19 11:21:31 Loop: 0, Int: 4, Dur(s): 1.0, Mode: DEL, Ops: 5604, MB/s: 21.89, IO/s: 5604, Lat(ms): [ min: 1.2, avg: 1.8, 99%: 2.7, max: 11.6 ], Slowdowns: 0
2019/08/19 11:21:32 Loop: 0, Int: 5, Dur(s): 1.0, Mode: DEL, Ops: 5610, MB/s: 21.91, IO/s: 5610, Lat(ms): [ min: 1.2, avg: 1.8, 99%: 2.8, max: 8.4 ], Slowdowns: 0
2019/08/19 11:21:33 Loop: 0, Int: 6, Dur(s): 1.0, Mode: DEL, Ops: 5526, MB/s: 21.59, IO/s: 5526, Lat(ms): [ min: 1.2, avg: 1.8, 99%: 2.8, max: 11.6 ], Slowdowns: 0
2019/08/19 11:21:34 Loop: 0, Int: 7, Dur(s): 1.0, Mode: DEL, Ops: 5538, MB/s: 21.63, IO/s: 5538, Lat(ms): [ min: 1.2, avg: 1.8, 99%: 2.8, max: 8.7 ], Slowdowns: 0
2019/08/19 11:21:35 Loop: 0, Int: TOTAL, Dur(s): 8.6, Mode: DEL, Ops: 47783, MB/s: 21.61, IO/s: 5532, Lat(ms): [ min: 1.1, avg: 1.8, 99%: 2.8, max: 67.1 ], Slowdowns: 0
2019/08/19 11:21:35 Running Loop 0 BUCKET DELETE TEST
2019/08/19 11:21:35 Loop: 0, Int: TOTAL, Dur(s): 0.0, Mode: BDEL, Ops: 10, MB/s: 0.00, IO/s: 250, Lat(ms): [ min: 35.1, avg: 37.1, 99%: 39.4, max: 39.4 ], Slowdowns: 0
```

One notable point is that like the s3-benchmark program it is based on, hsbench has relatively low CPU overhead compared to some other S3 benchmarks.  During the 4K PUT phase of the above test:

```
22628 root      20   0 6700416 5.308g  28072 S 413.3  8.4 676:28.86 ceph-osd
23329 root      20   0 1658700 253068  21320 S 333.3  0.4 530:06.76 radosgw
 9017 perf      20   0 2184580  33612   6568 S 173.3  0.1   0:05.25 hsbench
```
