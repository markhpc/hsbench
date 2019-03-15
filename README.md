# Introduction

## This is a fork version of [wasabi-tech/se-benchmark](https://github.com/wasabi-tech/s3-benchmark), to make it support minio and ceph.

Example output:

```
[wen@t s3-benchmark]$ . ceph.env 
[wen@t s3-benchmark]$ s3-benchmark 
Wasabi benchmark program v2.0
Parameters: url=http://s3.t.haodai.net, bucket=loadgen, region=us-east-1, duration=60, threads=1, loops=1, size=1M
Loop 1: PUT time 60.1 secs, objects = 297, speed = 4.9MB/sec, 4.9 operations/sec. Slowdowns = 0
Loop 1: GET time 7.8 secs, objects = 297, speed = 38.2MB/sec, 38.2 operations/sec. Slowdowns = 0
Loop 1: DELETE time 24.2 secs, 12.3 deletes/sec. Slowdowns = 0
[wen@t s3-benchmark]$ . miniostest.env 
[wen@t s3-benchmark]$ s3-benchmark 
Wasabi benchmark program v2.0
Parameters: url=http://minio-cluster.tt.haodai.net, bucket=loadgen, region=us-east-1, duration=60, threads=1, loops=1, size=1M
Loop 1: PUT time 60.0 secs, objects = 576, speed = 9.6MB/sec, 9.6 operations/sec. Slowdowns = 0
Loop 1: GET time 9.4 secs, objects = 576, speed = 61.3MB/sec, 61.3 operations/sec. Slowdowns = 0
Loop 1: DELETE time 4.2 secs, 138.4 deletes/sec. Slowdowns = 0
[wen@t s3-benchmark]$ 
[wen@t s3-benchmark]$ . ceph.env 
[wen@t s3-benchmark]$ s3-benchmark -t 10
Wasabi benchmark program v2.0
Parameters: url=http://s3.t.haodai.net, bucket=loadgen, region=us-east-1, duration=60, threads=10, loops=1, size=1M
Loop 1: PUT time 60.9 secs, objects = 513, speed = 8.4MB/sec, 8.4 operations/sec. Slowdowns = 0
Loop 1: GET time 45.6 secs, objects = 5130, speed = 112.5MB/sec, 112.5 operations/sec. Slowdowns = 0
Loop 1: DELETE time 38.4 secs, 13.4 deletes/sec. Slowdowns = 0
[wen@t s3-benchmark]$ 
[wen@t s3-benchmark]$ . miniostest.env 
[wen@t s3-benchmark]$ s3-benchmark -t 10
Wasabi benchmark program v2.0
Parameters: url=http://minio-cluster.tt.haodai.net, bucket=loadgen, region=us-east-1, duration=60, threads=10, loops=1, size=1M
Loop 1: PUT time 62.4 secs, objects = 832, speed = 13.3MB/sec, 13.3 operations/sec. Slowdowns = 0
Loop 1: GET time 60.1 secs, objects = 6774, speed = 112.8MB/sec, 112.8 operations/sec. Slowdowns = 0
Loop 1: DELETE time 1.4 secs, 578.1 deletes/sec. Slowdowns = 0
[wen@t s3-benchmark]$ 
```
s3-benchmark is a performance testing tool provided by Wasabi for performing S3 operations (PUT, GET, and DELETE) for objects. Besides the bucket configuration, the object size and number of threads varied be given for different tests.

The testing tool is loosely based on the Nasuni (http://www6.nasuni.com/rs/nasuni/images/Nasuni-2015-State-of-Cloud-Storage-Report.pdf) performance benchmarking methodologies used to test the performance of different cloud storage providers

# Prerequisites
To leverage this tool, the following prerequisites apply:
*	Git development environment
*	Ubuntu Linux shell programming skills
*	Access to a Go 1.7 development system (only if the OS is not Ubuntu Linux 16.04)
*	Access to the appropriate AWS EC2 (or equivalent) compute resource (optimal performance is realized using m4.10xlarge EC2 Ubuntu with 10 GB ENA)


# Building the Program
Obtain a local copy of the repository using the following git command with any directory that is convenient:

```
git clone https://github.com/wasabi-tech/s3-benchmark.git
```

You should see the following files in the s3-benchmark directory.
LICENSE	README.md		s3-benchmark.go	s3-benchmark.ubuntu

If the test is being run on Ubuntu version 16.04 LTS (the current long term release), the binary
executable s3-benchmark.ubuntu will run the benchmark testing without having to build the executable. 

Otherwise, to build the s3-benchmark executable, you must issue this following command:
/usr/bin/go build s3-bechmark.go
 
# Command Line Arguments
Below are the command line arguments to the program (which can be displayed using -help):

```
  -a string
        Access key
  -b string
        Bucket for testing (default "wasabi-benchmark-bucket")
  -d int
        Duration of each test in seconds (default 60)
  -l int
        Number of times to repeat test (default 1)
  -s string
        Secret key
  -t int
        Number of threads to run (default 1)
  -u string
        URL for host with method prefix (default "http://s3.wasabisys.com")
  -z string
        Size of objects in bytes with postfix K, M, and G (default "1M")
```        

# Example Benchmark
Below is an example run of the benchmark for 10 threads with the default 1MB object size.  The benchmark reports
for each operation PUT, GET and DELETE the results in terms of data speed and operations per second.  The program
writes all results to the log file benchmark.log.

```
ubuntu:~/s3-benchmark$ ./s3-benchmark.ubuntu -a MY-ACCESS-KEY -b jeff-s3-benchmark -s MY-SECRET-KEY -t 10 
Wasabi benchmark program v2.0
Parameters: url=http://s3.wasabisys.com, bucket=jeff-s3-benchmark, duration=60, threads=10, loops=1, size=1M
Loop 1: PUT time 60.1 secs, objects = 5484, speed = 91.3MB/sec, 91.3 operations/sec.
Loop 1: GET time 60.1 secs, objects = 5483, speed = 91.3MB/sec, 91.3 operations/sec.
Loop 1: DELETE time 1.9 secs, 2923.4 deletes/sec.
Benchmark completed.
```

# Note
Your performance testing benchmark results may vary most often because of limitations of your network connection to the cloud storage provider.  Wasabi performance claims are tested under conditions that remove any latency (which can be shown using the ping command) and bandwidth bottlenecks that restrict how fast data can be moved.  For more information,
contact Wasabi technical support (support@wasabi.com).
