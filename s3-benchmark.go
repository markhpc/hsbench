// s3-benchmark.go
// Copyright (c) 2017 Wasabi Technology, Inc.

package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"code.cloudfoundry.org/bytefmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// Global variables
var access_key, secret_key, url_host, bucket_prefix, object_prefix, region, modes, sizeArg string
var buckets []string
var duration_secs, threads, loops int
var object_data []byte
var object_data_md5 string
var running_threads, bucket_count, object_count, object_size, op_counter int64
var endtime, upload_finish, download_finish, delete_finish time.Time
var interval float64

func logit(msg string) {
	fmt.Println(msg)
	logfile, _ := os.OpenFile("benchmark.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if logfile != nil {
		logfile.WriteString(time.Now().Format(http.TimeFormat) + ": " + msg + "\n")
		logfile.Close()
	}
}

// Our HTTP transport used for the roundtripper below
var HTTPTransport http.RoundTripper = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	Dial: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).Dial,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 0,
	// Allow an unlimited number of idle connections
	MaxIdleConnsPerHost: 4096,
	MaxIdleConns:        0,
	// But limit their idle time
	IdleConnTimeout: time.Minute,
	// Ignore TLS errors
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
}

var httpClient = &http.Client{Transport: HTTPTransport}

func getS3Client() *s3.S3 {
	// Build our config
	creds := credentials.NewStaticCredentials(access_key, secret_key, "")
	loglevel := aws.LogOff
	// Build the rest of the configuration
	awsConfig := &aws.Config{
		Region:               aws.String(region),
		Endpoint:             aws.String(url_host),
		Credentials:          creds,
		LogLevel:             &loglevel,
		S3ForcePathStyle:     aws.Bool(true),
		S3Disable100Continue: aws.Bool(true),
		// Comment following to use default transport
		HTTPClient: &http.Client{Transport: HTTPTransport},
	}
	session := session.New(awsConfig)
	client := s3.New(session)
	if client == nil {
		log.Fatalf("FATAL: Unable to create new client.")
	}
	// Return success
	return client
}

func createBucket(bucket_num int64, ignore_errors bool) {
	svc := s3.New(session.New(), cfg)
	in := &s3.CreateBucketInput{Bucket: aws.String(buckets[bucket_num])}
	if _, err := svc.CreateBucket(in); err != nil {
		if strings.Contains(err.Error(), s3.ErrCodeBucketAlreadyOwnedByYou) ||
			strings.Contains(err.Error(), "BucketAlreadyExists") {
			return
		}
		if ignore_errors {
			log.Printf("WARNING: createBucket %s error, ignoring %v", buckets[bucket_num], err)
		} else {
			log.Fatalf("FATAL: Unable to create bucket %s (is your access and secret correct?): %v", buckets[bucket_num], err)
		}
	}
}

func deleteAllObjects(bucket_num int64) {
	svc := s3.New(session.New(), cfg)
	out, err := svc.ListObjects(&s3.ListObjectsInput{Bucket: &buckets[bucket_num]})
	if err != nil {
		log.Fatal("can't list objects")
	}
	n := len(out.Contents)
	for n > 0 {
		fmt.Printf("got existing %v objects, try to delete now...\n", n)

		for _, v := range out.Contents {
			svc.DeleteObject(&s3.DeleteObjectInput{
				Bucket: &buckets[bucket_num],
				Key:    v.Key,
			})
		}
		out, err = svc.ListObjects(&s3.ListObjectsInput{Bucket: &buckets[bucket_num]})
		if err != nil {
			log.Fatal("can't list objects")
		}
		n = len(out.Contents)
		fmt.Printf("after delete, got %v objects\n", n)
	}
}

// canonicalAmzHeaders -- return the x-amz headers canonicalized
func canonicalAmzHeaders(req *http.Request) string {
	// Parse out all x-amz headers
	var headers []string
	for header := range req.Header {
		norm := strings.ToLower(strings.TrimSpace(header))
		if strings.HasPrefix(norm, "x-amz") {
			headers = append(headers, norm)
		}
	}
	// Put them in sorted order
	sort.Strings(headers)
	// Now add back the values
	for n, header := range headers {
		headers[n] = header + ":" + strings.Replace(req.Header.Get(header), "\n", " ", -1)
	}
	// Finally, put them back together
	if len(headers) > 0 {
		return strings.Join(headers, "\n") + "\n"
	} else {
		return ""
	}
}

func hmacSHA1(key []byte, content string) []byte {
	mac := hmac.New(sha1.New, key)
	mac.Write([]byte(content))
	return mac.Sum(nil)
}

func setSignature(req *http.Request) {
	// Setup default parameters
	dateHdr := time.Now().UTC().Format("20060102T150405Z")
	req.Header.Set("X-Amz-Date", dateHdr)
	// Get the canonical resource and header
	canonicalResource := req.URL.EscapedPath()
	canonicalHeaders := canonicalAmzHeaders(req)
	stringToSign := req.Method + "\n" + req.Header.Get("Content-MD5") + "\n" + req.Header.Get("Content-Type") + "\n\n" +
		canonicalHeaders + canonicalResource
	hash := hmacSHA1([]byte(secret_key), stringToSign)
	signature := base64.StdEncoding.EncodeToString(hash)
	req.Header.Set("Authorization", fmt.Sprintf("AWS %s:%s", access_key, signature))
}

type IntervalStats struct {
	bytes int64
	slowdowns int64
	latNano []int64
}

type ThreadStats struct {
	start int64 
	curInterval int64
	intervals []IntervalStats
}

func makeThreadStats(s int64, intervalNano int64) ThreadStats {
	ts := ThreadStats{start: s, curInterval: -1}
	ts.updateIntervals(intervalNano)
	return ts
}

func (ts *ThreadStats) updateIntervals(intervalNano int64) int64 {
	if intervalNano < 0 {
		return ts.curInterval
	}
	for ts.start + intervalNano*ts.curInterval < time.Now().UnixNano() {
		ts.intervals = append(ts.intervals, IntervalStats{0, 0, []int64{}})
		ts.curInterval++
	}
	return ts.curInterval
}

func (ts *ThreadStats) finish() {
	ts.curInterval = -1
}

type Stats struct {
	// threads
	threads int
	// The loop we are in
	loop int
	// Test mode being run 
        mode string
	// start time in nanoseconds
        startNano int64
	// end time in nanoseconds
	endNano int64
	// Duration in nanoseconds for each interval
	intervalNano int64
	// Per-thread statistics
	threadStats []ThreadStats
	// a map of counters of how many threads have finished given interval of stats
	intervalCompletions sync.Map 
	// a counter of how many threads have finished updating stats entirely
	completions int32
}

func makeStats(loop int, mode string, threads int, intervalNano int64) Stats {
	start := time.Now().UnixNano()
	s := Stats{threads, loop, mode, start, 0, intervalNano, []ThreadStats{}, sync.Map{}, 0}
	for i := 0; i < threads; i++ {
		s.threadStats = append(s.threadStats, makeThreadStats(start, s.intervalNano))
	}
	return s
}

func (stats *Stats) _getIntervalStats(i int64) IntervalStats {
        bytes := int64(0)
        ops := int64(0)
        slowdowns := int64(0);

        for t := 0; t < stats.threads; t++ {
                bytes += stats.threadStats[t].intervals[i].bytes
                ops += int64(len(stats.threadStats[t].intervals[i].latNano))
                slowdowns += stats.threadStats[t].intervals[i].slowdowns
        }
        // Aggregate the per-thread Latency slice
        tmpLat := make([]int64, ops)
        var c int
        for t := 0; t < stats.threads; t++ {
                c += copy(tmpLat[c:], stats.threadStats[t].intervals[i].latNano)
        }
        sort.Slice(tmpLat, func(i, j int) bool { return tmpLat[i] < tmpLat[j] })
        return IntervalStats{bytes, slowdowns, tmpLat}
}

func (stats *Stats) _getTotalStats() IntervalStats {
        bytes := int64(0)
        ops := int64(0)
        slowdowns := int64(0);

	for t := 0; t < stats.threads; t++ {
		for i := 0; i < len(stats.threadStats[t].intervals); i++ {
	                bytes += stats.threadStats[t].intervals[i].bytes
	                ops += int64(len(stats.threadStats[t].intervals[i].latNano))
	                slowdowns += stats.threadStats[t].intervals[i].slowdowns
	        }
	}
        // Aggregate the per-thread Latency slice
        tmpLat := make([]int64, ops)
        var c int 
        for t := 0; t < stats.threads; t++ {
	        for i := 0; i < len(stats.threadStats[t].intervals); i++ {
	                c += copy(tmpLat[c:], stats.threadStats[t].intervals[i].latNano)
	        }
	}
        sort.Slice(tmpLat, func(i, j int) bool { return tmpLat[i] < tmpLat[j] })
        return IntervalStats{bytes, slowdowns, tmpLat}
}

func (stats *Stats) logI(i int64) bool {
	// Check bounds first
	if stats.intervalNano < 0 || i < 0 {
		return false;
	}
	// Not safe to log if not all writers have completed.
	value, ok := stats.intervalCompletions.Load(i)
	if !ok {
		return false;
	}
	cp, ok := value.(*int32)
	if !ok {
		return false;
	}
	count := atomic.LoadInt32(cp)
	if count < int32(stats.threads) {
		return false;
	}

	return stats._log(strconv.FormatInt(i, 10), stats.intervalNano, stats._getIntervalStats(i)) 
}

func (stats *Stats) log() bool {
	// Not safe to log if not all writers have completed.
	completions := atomic.LoadInt32(&stats.completions)
        if (completions < int32(stats.threads)) {
		log.Printf("log, completions: %d", completions) 
                return false;
        }
        return stats._log("ALL", stats.endNano - stats.startNano, stats._getTotalStats())
}

func (stats *Stats) _log(intervalName string, intervalNano int64, intervalStats IntervalStats) bool {
        // Compute and log the stats
	ops := len(intervalStats.latNano)
        totalLat := int64(0);
	minLat := float64(0);
	maxLat := float64(0);
	NinetyNineLat := float64(0);
	avgLat := float64(0);
	if ops > 0 {
		minLat = float64(intervalStats.latNano[0]) / 1000000
		maxLat = float64(intervalStats.latNano[ops - 1]) / 1000000
		for i := range intervalStats.latNano {
			totalLat += intervalStats.latNano[i]
		}
		avgLat = float64(totalLat) / float64(ops) / 1000000
		NintyNineLatNano := intervalStats.latNano[int64(math.Round(0.99*float64(ops))) - 1]
		NinetyNineLat = float64(NintyNineLatNano) / 1000000
	}
	seconds := float64(intervalNano) / 1000000000
	mbps := float64(intervalStats.bytes) / seconds / bytefmt.MEGABYTE
	iops := float64(ops) / seconds

	log.Printf(
		"Loop: %d, Int: %s, Dur(s): %.1f, Mode: %s, Ops: %d, MB/s: %.2f, IO/s: %.0f, Lat(ms): [ min: %.1f, avg: %.1f, 99%%: %.1f, max: %.1f ], Slowdowns: %d",
		stats.loop,
		intervalName,
                seconds,
		stats.mode,
		ops,
		mbps,
		iops,
		minLat,
		avgLat,
		NinetyNineLat,
		maxLat,
		intervalStats.slowdowns)
	return true
}

// Only safe to call from the calling thread
func (stats *Stats) updateIntervals(thread_num int) int64 {
	curInterval := stats.threadStats[thread_num].curInterval
	newInterval := stats.threadStats[thread_num].updateIntervals(stats.intervalNano)

	// Finish has already been called
	if curInterval < 0 {
		return -1
	}

	for i := curInterval; i < newInterval; i++ {
		// load or store the current value
		value, _ := stats.intervalCompletions.LoadOrStore(i, new(int32))
		cp, ok := value.(*int32)
		if !ok {
			log.Printf("updateIntervals: got data of type %T but wanted *int32", value)
			continue
		}

		count := atomic.AddInt32(cp, 1)
		if count == int32(stats.threads) {
			stats.logI(i)
		}
	}
	return newInterval
}

func (stats *Stats) addOp(thread_num int, bytes int64, latNano int64) {

	// Interval statistics
	cur := stats.threadStats[thread_num].curInterval
	if cur < 0 {
		return
	}
	stats.threadStats[thread_num].intervals[cur].bytes += bytes
	stats.threadStats[thread_num].intervals[cur].latNano =
		append(stats.threadStats[thread_num].intervals[cur].latNano, latNano)
}

func (stats *Stats) addSlowDown(thread_num int) {
	cur := stats.threadStats[thread_num].curInterval
	stats.threadStats[thread_num].intervals[cur].slowdowns++
}

func (stats *Stats) finish(thread_num int) {
	stats.threadStats[thread_num].updateIntervals(stats.intervalNano)
	stats.threadStats[thread_num].finish()
	count := atomic.AddInt32(&stats.completions, 1)
	if count ==  int32(stats.threads) {
		stats.endNano = time.Now().UnixNano()
	}
}

func runUpload(thread_num int, fendtime time.Time, stats *Stats) {
	errcnt := 0
	svc := s3.New(session.New(), cfg)
	for {
		if duration_secs > -1 && time.Now().After(endtime) {
			break
		}
                objnum := atomic.AddInt64(&op_counter, 1)
                bucket_num := objnum % int64(bucket_count)
		if object_count > -1 && objnum >= object_count {
			objnum = atomic.AddInt64(&op_counter, -1)
			break
		}
		fileobj := bytes.NewReader(object_data)

                key := fmt.Sprintf("%s%012d", object_prefix, objnum)
		r := &s3.PutObjectInput{
			Bucket: &buckets[bucket_num],
			Key:    &key,
			Body:   fileobj,
		}
		start := time.Now().UnixNano()
		req, _ := svc.PutObjectRequest(r)
		// Disable payload checksum calculation (very expensive)
		req.HTTPRequest.Header.Add("X-Amz-Content-Sha256", "UNSIGNED-PAYLOAD")
		err := req.Send()
                end := time.Now().UnixNano()
                stats.updateIntervals(thread_num)

		if err != nil {
			errcnt++
                        stats.addSlowDown(thread_num);
			atomic.AddInt64(&op_counter, -1)
			fmt.Println("upload err", err)
		} else {
			// Update the stats
			stats.addOp(thread_num, object_size, end-start)
		}
		if errcnt > 2 {
			break
		}
	}
	// Remember last done time
	upload_finish = time.Now()
	// One less thread
	atomic.AddInt64(&running_threads, -1)
	// stats are done
	stats.finish(thread_num)
}

func runDownload(thread_num int, fendtime time.Time, stats *Stats) {
	errcnt := 0
	svc := s3.New(session.New(), cfg)
	for {
		if duration_secs > -1 && time.Now().After(endtime) {
                        break
                }

		objnum := atomic.AddInt64(&op_counter, 1)
                if objnum >= object_count {
			atomic.AddInt64(&op_counter, -1)
                        break
                }

		bucket_num := objnum % int64(bucket_count)
                key := fmt.Sprintf("%s%012d", object_prefix, objnum)
                r := &s3.GetObjectInput{
                        Bucket: &buckets[bucket_num],
                        Key:    &key,
                }

		start := time.Now().UnixNano()
                req, resp := svc.GetObjectRequest(r)
                err := req.Send()
		end := time.Now().UnixNano()
		stats.updateIntervals(thread_num)

                if err != nil {
                        errcnt++
			stats.addSlowDown(thread_num);
                        fmt.Println("download err", err)
		} else {
			// Update the stats
			stats.addOp(thread_num, object_size, end-start)
		}

                if err == nil {
                        _, err = io.Copy(ioutil.Discard, resp.Body)
                }
                if errcnt > 2 {
                       break 
                }

	}
	// Remember last done time
	download_finish = time.Now()
	// One less thread
	atomic.AddInt64(&running_threads, -1)
	// stats are done
	stats.finish(thread_num)
}

func runDelete(thread_num int, stats *Stats) {
	errcnt := 0
	svc := s3.New(session.New(), cfg)

	for {
                objnum := atomic.AddInt64(&op_counter, 1)
		if objnum >= object_count {
                        atomic.AddInt64(&op_counter, -1)
			break
		}

                bucket_num := objnum % int64(bucket_count)

                key := fmt.Sprintf("%s%012d", object_prefix, objnum)
                r := &s3.DeleteObjectInput{
                        Bucket: &buckets[bucket_num],
                        Key:    &key,
                }

		start := time.Now().UnixNano()
                req, out := svc.DeleteObjectRequest(r)
                err := req.Send()
		end := time.Now().UnixNano()
		stats.updateIntervals(thread_num)

                if err != nil {
                        errcnt++
                        stats.addSlowDown(thread_num);
                        fmt.Println("delete err", err, "out", out.String())
                } else {
			// Update the stats
			stats.addOp(thread_num, object_size, end-start)
		}
                if errcnt > 2 {
			break
                }
	}
	// Remember last done time
	delete_finish = time.Now()
	// One less thread
	atomic.AddInt64(&running_threads, -1)
        // stats are done
        stats.finish(thread_num)
}

var cfg *aws.Config

func initBuckets(thread_num int, stats *Stats) {
	// Create the buckets and delete all the objects
	for {
		bucket_num := atomic.AddInt64(&op_counter, 1)
		if bucket_num >= bucket_count {
			atomic.AddInt64(&op_counter, -1)
			break
		}
                start := time.Now().UnixNano()
                createBucket(bucket_num, true)
                deleteAllObjects(bucket_num)
                end := time.Now().UnixNano()
                stats.updateIntervals(thread_num)
                stats.addOp(thread_num, 0, end-start)
	}
        atomic.AddInt64(&running_threads, -1)
	stats.finish(thread_num)
}

func runWrapper(loop int, r rune) {
	op_counter = -1 
        running_threads = int64(threads)
        intervalNano := int64(interval*1000000000)
        endtime = time.Now().Add(time.Second * time.Duration(duration_secs))
	var stats Stats

	switch r {
	case 'i':
		log.Printf("Running Loop %d Init", loop)
		stats = makeStats(loop, "INIT", threads, intervalNano)
		for n := 0; n < threads; n++ {
			go initBuckets(n, &stats);
		}
	case 'p':
		log.Printf("Running Loop %d Put Test", loop)
                stats = makeStats(loop, "PUT", threads, intervalNano)
		for n := 0; n < threads; n++ {
			go runUpload(n, endtime, &stats);
		}
       	case 'g':
                log.Printf("Running Loop %d Get Test", loop)
                stats = makeStats(loop, "GET", threads, intervalNano)
              	for n := 0; n < threads; n++ {
                       	go runDownload(n, endtime, &stats);
		}
       	case 'd':
                log.Printf("Running Loop %d Del Test", loop)
                stats = makeStats(loop, "DEL", threads, intervalNano)
               	for n := 0; n < threads; n++ {
                       	go runDelete(n, &stats);
               	}
	}
        // Wait for it to finish
        for atomic.LoadInt64(&running_threads) > 0 {
                time.Sleep(time.Millisecond)
        }
        stats.log()
}

func init() {
	// Parse command line
	myflag := flag.NewFlagSet("myflag", flag.ExitOnError)
	myflag.StringVar(&access_key, "a", os.Getenv("AWS_ACCESS_KEY_ID"), "Access key")
	myflag.StringVar(&secret_key, "s", os.Getenv("AWS_SECRET_ACCESS_KEY"), "Secret key")
	myflag.StringVar(&url_host, "u", os.Getenv("AWS_HOST"), "URL for host with method prefix")
        myflag.StringVar(&object_prefix, "op", "", "Prefix for objects")
	myflag.StringVar(&bucket_prefix, "bp", "hotsauce_bench", "Prefix for buckets")
	myflag.StringVar(&region, "r", "us-east-1", "Region for testing")
        myflag.StringVar(&modes, "m", "ipgd", "Run modes in order.  See NOTES for more info")
        myflag.Int64Var(&object_count, "n", -1, "Maximum number of objects <-1 for unlimited>")
        myflag.Int64Var(&bucket_count, "b", 1, "Number of buckets to distribute IOs across")
	myflag.IntVar(&duration_secs, "d", 60, "Maximum test duration in seconds <-1 for unlimited>")
	myflag.IntVar(&threads, "t", 1, "Number of threads to run")
	myflag.IntVar(&loops, "l", 1, "Number of times to repeat test")
	myflag.StringVar(&sizeArg, "z", "1M", "Size of objects in bytes with postfix K, M, and G")
	myflag.Float64Var(&interval, "ri", 1.0, "Number of seconds between report intervals")
	// define custom usage output with notes
        notes :=
`
NOTES:
  - Valid mode types for the -m mode string are:
    i: initialize buckets and clear any existing objects
    p: put objects in buckets
    g: get objects from buckets
    d: delete objects from buckets

    These modes are processed in-order and can be repeated, ie "ippgd" will
    initialize the buckets, put the objects, reput the objects, get the
    objects, and then delete the objects.  The repeat flag will repeat this
    whole process the specified number of times.
`
	myflag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "\nUSAGE: %s [OPTIONS]\n\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "OPTIONS:\n")
		myflag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), notes);
	}

	if err := myflag.Parse(os.Args[1:]); err != nil {
		os.Exit(1)
	}

	// Check the arguments
	if object_count < 0 && duration_secs < 0 {
		log.Fatal("The number of objects and duration can not both be unlimited")
	}
	if access_key == "" {
		log.Fatal("Missing argument -a for access key.")
	}
	if secret_key == "" {
		log.Fatal("Missing argument -s for secret key.")
	}
	if url_host == "" {
		log.Fatal("Missing argument -s for host endpoint.")
	}
	invalid_mode := false
	for _, r := range modes {
		if (r != 'i' && r != 'p' && r != 'g' && r != 'd') {
			s := fmt.Sprintf("Invalid mode '%s' passed to -m", string(r))
			log.Printf(s)
			invalid_mode = true
		}
	}
	if invalid_mode {
		log.Fatal("Invalid modes passed to -m, see help for details.")
	}	
	var err error
	var size uint64
	if size, err = bytefmt.ToBytes(sizeArg); err != nil {
		log.Fatalf("Invalid -z argument for object size: %v", err)
	}
	object_size = int64(size)
}

func initData() {
        // Initialize data for the bucket
        object_data = make([]byte, object_size)
        rand.Read(object_data)
        hasher := md5.New()
        hasher.Write(object_data)
        object_data_md5 = base64.StdEncoding.EncodeToString(hasher.Sum(nil))
}

func main() {
	// Hello
	fmt.Println("Wasabi benchmark program v2.0")

	//fmt.Println("accesskey:", access_key, "secretkey:", secret_key)
	cfg = &aws.Config{
		Endpoint:    aws.String(url_host),
		Credentials: credentials.NewStaticCredentials(access_key, secret_key, ""),
		Region:      aws.String(region),
		// DisableParamValidation:  aws.Bool(true),
		DisableComputeChecksums: aws.Bool(true),
		S3ForcePathStyle:        aws.Bool(true),
	}

	// Echo the parameters
	logit(fmt.Sprintf("Parameters: url=%s, bucket_prefix=%s, bucket_count=%d, region=%s, duration=%d, threads=%d, loops=%d, size=%s",
		url_host, bucket_prefix, bucket_count, region, duration_secs, threads, loops, sizeArg))

	// Init Data
	initData()

	// Setup the slice of buckets
        for i := int64(0); i < bucket_count; i++ {
		buckets = append(buckets, fmt.Sprintf("%s%012d", bucket_prefix, i))
	}

	// Loop running the tests
	for loop := 0; loop < loops; loop++ {
	        for _, r := range modes {
			runWrapper(loop, r)
		}
	}
}
