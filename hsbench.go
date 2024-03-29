// hsbench.go
// Copyright (c) 2017 Wasabi Technology, Inc.
// Copyright (c) 2019 Red Hat Inc.

package main

import (
	"code.cloudfoundry.org/bytefmt"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pkg/errors"
	"github.com/vponomarev/hsbench/internal"
	"hash/fnv"
	"io"
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
)

const (
	ErrContextRequestCancelled = "request context canceled"
	ErrContextDeadlineExceeded = "context deadline exceeded"
)

// Global variables
var access_key, secret_key, url_host, bucket_prefix, bucket_list, object_prefix, region, modes, storage_class, output, json_output, minSizeArg, sizeArg string
var op_timeout int64
var ranged_offset int64
var ranged_size int64
var objects_info_output string
var buckets []string
var duration_secs, threads, loops int
var max_keys, running_threads, bucket_count, first_object, object_count, object_max_size, object_min_size, op_counter int64
var object_count_flag bool
var endtime time.Time
var interval float64
var object_info_chan chan ObjectInfo
var response_statuses map[string]int
var response_statuses_mu sync.Mutex
var app_context context.Context
var workload_profile_file string
var workload_profile_name string
var workload_config WorkloadConfig

func processAWSError(err error) {
	if err == nil {
		return
	}

	response_statuses_mu.Lock()
	defer response_statuses_mu.Unlock()
	if aerr, ok := err.(awserr.Error); ok {
		response_statuses[aerr.Code()]++
	} else {
		response_statuses[err.Error()]++
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
	// Set the number of idle connections to 2X the number of threads
	MaxIdleConnsPerHost: 2 * threads,
	MaxIdleConns:        2 * threads,
	// But limit their idle time to 1 minute
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
	loop         int
	name         string
	mode         string
	bytes        int64
	slowdowns    int64
	intervalNano int64
	latNano      []int64
}

func (is *IntervalStats) makeOutputStats() OutputStats {
	// Compute and log the stats
	ops := len(is.latNano)
	totalLat := int64(0)
	minLat := float64(0)
	maxLat := float64(0)
	NinetyNineLat := float64(0)
	avgLat := float64(0)
	if ops > 0 {
		minLat = float64(is.latNano[0]) / 1000000
		maxLat = float64(is.latNano[ops-1]) / 1000000
		for i := range is.latNano {
			totalLat += is.latNano[i]
		}
		avgLat = float64(totalLat) / float64(ops) / 1000000
		NintyNineLatNano := is.latNano[int64(math.Round(0.99*float64(ops)))-1]
		NinetyNineLat = float64(NintyNineLatNano) / 1000000
	}
	seconds := float64(is.intervalNano) / 1000000000
	mbps := float64(is.bytes) / seconds / bytefmt.MEGABYTE
	iops := float64(ops) / seconds

	response_statuses_mu.Lock()
	statuses := make(map[string]int, len(response_statuses))
	for status, count := range response_statuses {
		statuses[status] = count
	}

	// Clear statuses map
	response_statuses = make(map[string]int, 50)
	response_statuses_mu.Unlock()

	return OutputStats{
		is.loop,
		is.name,
		seconds,
		is.mode,
		ops,
		mbps,
		iops,
		minLat,
		avgLat,
		NinetyNineLat,
		maxLat,
		is.slowdowns,
		statuses}
}

type OutputStats struct {
	Loop          int
	IntervalName  string
	Seconds       float64
	Mode          string
	Ops           int
	Mbps          float64
	Iops          float64
	MinLat        float64
	AvgLat        float64
	NinetyNineLat float64
	MaxLat        float64
	Slowdowns     int64
	statuses      map[string]int
}

func (o *OutputStats) log() {
	log.Printf(
		"Loop: %d, Int: %s, Dur(s): %.1f, Mode: %s, Ops: %d, MB/s: %.2f, IO/s: %.0f, Lat(ms): [ min: %.1f, avg: %.1f, 99%%: %.1f, max: %.1f ], Slowdowns: %d, Statuses: %v",
		o.Loop,
		o.IntervalName,
		o.Seconds,
		o.Mode,
		o.Ops,
		o.Mbps,
		o.Iops,
		o.MinLat,
		o.AvgLat,
		o.NinetyNineLat,
		o.MaxLat,
		o.Slowdowns,
		o.statuses)
}

func (o *OutputStats) csv_header(w *csv.Writer) {
	if w == nil {
		log.Fatal("OutputStats passed nil CSV writer")
	}

	s := []string{
		"Loop",
		"Inteval",
		"Duration(s)",
		"Mode", "Ops",
		"MB/s",
		"IO/s",
		"Min Latency (ms)",
		"Avg Latency(ms)",
		"99% Latency(ms)",
		"Max Latency(ms)",
		"Slowdowns"}

	if err := w.Write(s); err != nil {
		log.Fatal("Error writing to CSV writer: ", err)
	}
}

func (o *OutputStats) csv(w *csv.Writer) {
	if w == nil {
		log.Fatal("OutputStats Passed nil csv writer")
	}

	s := []string{
		strconv.Itoa(o.Loop),
		o.IntervalName,
		strconv.FormatFloat(o.Seconds, 'f', 2, 64),
		o.Mode,
		strconv.Itoa(o.Ops),
		strconv.FormatFloat(o.Mbps, 'f', 2, 64),
		strconv.FormatFloat(o.Iops, 'f', 2, 64),
		strconv.FormatFloat(o.MinLat, 'f', 2, 64),
		strconv.FormatFloat(o.AvgLat, 'f', 2, 64),
		strconv.FormatFloat(o.NinetyNineLat, 'f', 2, 64),
		strconv.FormatFloat(o.MaxLat, 'f', 2, 64),
		strconv.FormatInt(o.Slowdowns, 10)}

	if err := w.Write(s); err != nil {
		log.Fatal("Error writing to CSV writer: ", err)
	}
}

func (o *OutputStats) json(jfile *os.File) {
	if jfile == nil {
		log.Fatal("OutputStats passed nil JSON file")
	}
	jdata, err := json.Marshal(o)
	if err != nil {
		log.Fatal("Error marshaling JSON: ", err)
	}
	log.Println(string(jdata))
	_, err = jfile.WriteString(string(jdata) + "\n")
	if err != nil {
		log.Fatal("Error writing to JSON file: ", err)
	}
}

type ThreadStats struct {
	start       int64
	curInterval int64
	intervals   []IntervalStats
}

func makeThreadStats(s int64, loop int, mode string, intervalNano int64) ThreadStats {
	ts := ThreadStats{s, 0, []IntervalStats{}}
	ts.intervals = append(ts.intervals, IntervalStats{loop, "0", mode, 0, 0, intervalNano, []int64{}})
	return ts
}

func (ts *ThreadStats) updateIntervals(loop int, mode string, intervalNano int64) int64 {
	// Interval statistics disabled, so just return the current interval
	if intervalNano < 0 {
		return ts.curInterval
	}
	for ts.start+intervalNano*(ts.curInterval+1) < time.Now().UnixNano() {
		ts.curInterval++
		ts.intervals = append(
			ts.intervals,
			IntervalStats{
				loop,
				strconv.FormatInt(ts.curInterval, 10),
				mode,
				0,
				0,
				intervalNano,
				[]int64{}})
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
	// a map of per-interval thread completion counters
	intervalCompletions sync.Map
	// a counter of how many threads have finished updating stats entirely
	completions int32
}

func makeStats(loop int, mode string, threads int, intervalNano int64) Stats {
	start := time.Now().UnixNano()
	s := Stats{threads, loop, mode, start, 0, intervalNano, []ThreadStats{}, sync.Map{}, 0}
	for i := 0; i < threads; i++ {
		s.threadStats = append(s.threadStats, makeThreadStats(start, s.loop, s.mode, s.intervalNano))
		s.updateIntervals(i)
	}
	return s
}

func (stats *Stats) makeOutputStats(i int64) (OutputStats, bool) {
	// Check bounds first
	if stats.intervalNano < 0 || i < 0 {
		return OutputStats{}, false
	}
	// Not safe to log if not all writers have completed.
	value, ok := stats.intervalCompletions.Load(i)
	if !ok {
		return OutputStats{}, false
	}
	cp, ok := value.(*int32)
	if !ok {
		return OutputStats{}, false
	}
	count := atomic.LoadInt32(cp)
	if count < int32(stats.threads) {
		return OutputStats{}, false
	}

	bytes := int64(0)
	ops := int64(0)
	slowdowns := int64(0)

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
	is := IntervalStats{stats.loop, strconv.FormatInt(i, 10), stats.mode, bytes, slowdowns, stats.intervalNano, tmpLat}
	return is.makeOutputStats(), true
}

func (stats *Stats) makeTotalStats() (OutputStats, bool) {
	// Not safe to log if not all writers have completed.
	completions := atomic.LoadInt32(&stats.completions)
	if completions < int32(threads) {
		log.Printf("log, completions: %d", completions)
		return OutputStats{}, false
	}

	bytes := int64(0)
	ops := int64(0)
	slowdowns := int64(0)

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
	is := IntervalStats{stats.loop, "TOTAL", stats.mode, bytes, slowdowns, stats.endNano - stats.startNano, tmpLat}
	return is.makeOutputStats(), true
}

// Only safe to call from the calling thread
func (stats *Stats) updateIntervals(thread_num int) int64 {
	curInterval := stats.threadStats[thread_num].curInterval
	newInterval := stats.threadStats[thread_num].updateIntervals(stats.loop, stats.mode, stats.intervalNano)

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
			if is, ok := stats.makeOutputStats(i); ok {
				is.log()
			}
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
	stats.updateIntervals(thread_num)
	stats.threadStats[thread_num].finish()
	count := atomic.AddInt32(&stats.completions, 1)
	if count == int32(stats.threads) {
		stats.endNano = time.Now().UnixNano()
	}
}

func generateSizeForObject() int64 {
	if object_min_size == 0 {
		return object_max_size
	}
	return object_min_size + rand.Int63n(object_max_size-object_min_size+1)
}

func runUpload(thread_num int, fendtime time.Time, stats *Stats) {
	iterator := int64(-1)
	errcnt := 0
	svcL := GetS3Services("")

	for {
		if duration_secs > -1 && time.Now().After(endtime) {
			break
		}
		iterator++

		objnum := atomic.AddInt64(&op_counter, 1)
		bucket_num := objnum % int64(bucket_count)
		if object_count > -1 && objnum >= object_count {
			objnum = atomic.AddInt64(&op_counter, -1)
			break
		}
		objectLen := generateSizeForObject()

		key := fmt.Sprintf("%s%012d", object_prefix, objnum)
		ts := time.Now()
		ts_seed := uint64(ts.UnixMilli())
		seed := generateSeed(key, ts_seed)
		fileobj := internal.NewRandomReadSeeker(seed, objectLen)

		r := &s3.PutObjectInput{
			Bucket: &buckets[bucket_num],
			Key:    &key,
			Body:   fileobj,
		}
		if storage_class != "" {
			r.StorageClass = &storage_class
		}
		start := time.Now().UnixNano()
		req, _ := svcL[iterator%int64(len(svcL))].PutObjectRequest(r)

		// Set up operation timeout if requested
		if op_timeout > 0 {
			ctx, _ := context.WithTimeout(app_context, time.Duration(op_timeout)*time.Millisecond)
			req.HTTPRequest = req.HTTPRequest.Clone(ctx)
		}

		// Disable payload checksum calculation (very expensive)
		req.HTTPRequest.Header.Add("X-Amz-Content-Sha256", "UNSIGNED-PAYLOAD")
		err := req.Send()
		end := time.Now().UnixNano()
		stats.updateIntervals(thread_num)

		errText := ""
		if err != nil {
			errText = err.Error()
		}
		object_info_chan <- ObjectInfo{
			Bucket:   buckets[bucket_num],
			Key:      key,
			Created:  ts_seed,
			Size:     objectLen,
			Duration: time.Duration(end-start) / 1000,
			Error:    errText,
		}

		processAWSError(err)
		if err != nil {
			errcnt++
			stats.addSlowDown(thread_num)
			atomic.AddInt64(&op_counter, -1)
			// fmt.Println(err.Error())
			if !strings.Contains(err.Error(), ErrContextRequestCancelled) {
				log.Printf("upload err", err)
			}
		} else {
			// Update the stats
			stats.addOp(thread_num, int64(objectLen), end-start)
		}
		//if errcnt > 2 {
		//	log.Printf("Too much errors, breaking")
		//	break
		//}
	}
	stats.finish(thread_num)
	atomic.AddInt64(&running_threads, -1)
}

func readBody(r io.Reader) (int64, error) {
	var bytesRead int64 = 0
	buf := make([]byte, 65536)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			bytesRead += int64(n)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return bytesRead, nil
			} else {
				return bytesRead, err
			}
		}
	}
}

func runDownload(thread_num int, fendtime time.Time, stats *Stats) {
	iterator := int64(-1)
	errcnt := 0
	svcL := GetS3Services("")

	for {
		if duration_secs > -1 && time.Now().After(endtime) {
			break
		}

		iterator++
		wpe := workload_config.GetEntry(0, iterator)

		var objnum int64
		if object_count > -1 {
			// Run random download if the number of objects is known
			objnum = first_object + rand.Int63()%object_count
		} else {
			objnum = atomic.AddInt64(&op_counter, 1)
			if object_count > -1 && objnum >= object_count {
				atomic.AddInt64(&op_counter, -1)
				break
			}
		}

		bucketNum := objnum % int64(bucket_count)
		bucketName := buckets[bucketNum]
		if wpe.Bucket != "" {
			bucketName = wpe.Bucket
		}

		key := fmt.Sprintf("%s%012d", object_prefix, objnum)
		r := &s3.GetObjectInput{
			Bucket: &bucketName,
			Key:    &key,
		}

		// Ranged get request

		rr := ""
		if wpe.Size > 0 {
			rr = fmt.Sprintf("bytes=%d-%d", wpe.Offset, wpe.Offset+wpe.Size)
			r.Range = &rr
		} else if wpe.Offset > 0 {
			rr = fmt.Sprintf("bytes=%d-", wpe.Offset)
			r.Range = &rr
		}

		start := time.Now().UnixNano()
		// req, resp := svc.GetObjectRequest(r)
		req, resp := svcL[iterator%int64(len(svcL))].GetObjectRequest(r)

		// Set up operation timeout if requested
		if op_timeout > 0 {
			ctx, _ := context.WithTimeout(app_context, time.Duration(op_timeout)*time.Millisecond)
			req.HTTPRequest = req.HTTPRequest.Clone(ctx)
		}

		err := req.Send()
		end := time.Now().UnixNano()
		stats.updateIntervals(thread_num)

		processAWSError(err)
		if err != nil {
			errcnt++
			stats.addSlowDown(thread_num)

			if !strings.Contains(err.Error(), ErrContextDeadlineExceeded) {
				log.Printf("download err: obj=[%s], err=[%v]", key, err)
			}
		} else {
			var bytesRead int64 = 0
			bytesRead, err2 := readBody(resp.Body)
			resp.Body.Close()

			if err2 != nil {
				if !strings.Contains(err2.Error(), ErrContextDeadlineExceeded) {
					log.Printf("download err during reading body", err2)
				}
			} else {
				// Update request finish time
				end = time.Now().UnixNano()

				// Update the stats
				if resp.ContentLength != nil {
					if *resp.ContentLength != bytesRead {
						log.Printf("downloaded %d bytes but content length is %d\n", bytesRead, *resp.ContentLength)
						err = io.ErrUnexpectedEOF
					}
				}
			}
			stats.addOp(thread_num, bytesRead, end-start)
			if err != nil {
				errcnt++
				stats.addSlowDown(thread_num)
				log.Printf("download err", err)
			}
		}
	}
	stats.finish(thread_num)
	atomic.AddInt64(&running_threads, -1)
}

func runDelete(thread_num int, stats *Stats) {
	iterator := int64(-1)
	errcnt := 0
	svcL := GetS3Services("")

	for {
		iterator++
		if duration_secs > -1 && time.Now().After(endtime) {
			break
		}

		objnum := atomic.AddInt64(&op_counter, 1)
		if object_count > -1 && objnum >= object_count {
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
		req, out := svcL[iterator%int64(len(svcL))].DeleteObjectRequest(r)
		err := req.Send()
		end := time.Now().UnixNano()
		stats.updateIntervals(thread_num)

		processAWSError(err)
		if err != nil {
			errcnt++
			stats.addSlowDown(thread_num)
			log.Printf("delete err", err, "out", out.String())
		} else {
			// Update the stats
			stats.addOp(thread_num, object_max_size, end-start)
		}
		if errcnt > 2 {
			break
		}
	}
	stats.finish(thread_num)
	atomic.AddInt64(&running_threads, -1)
}

func runBucketDelete(thread_num int, stats *Stats) {
	svc := s3.New(session.New(), cfg)

	for {
		bucket_num := atomic.AddInt64(&op_counter, 1)
		if bucket_num >= bucket_count {
			atomic.AddInt64(&op_counter, -1)
			break
		}
		r := &s3.DeleteBucketInput{
			Bucket: &buckets[bucket_num],
		}

		start := time.Now().UnixNano()
		_, err := svc.DeleteBucket(r)
		end := time.Now().UnixNano()
		stats.updateIntervals(thread_num)

		if err != nil {
			break
		}
		stats.addOp(thread_num, 0, end-start)
	}
	stats.finish(thread_num)
	atomic.AddInt64(&running_threads, -1)
}

func runBucketList(thread_num int, stats *Stats) {
	svc := s3.New(session.New(), cfg)

	marker := ""
	bucket_num := rand.Int63() % bucket_count
	for {
		if duration_secs > -1 && time.Now().After(endtime) {
			break
		}

		start := time.Now().UnixNano()
		p, err := svc.ListObjects(&s3.ListObjectsInput{
			Bucket:  &buckets[bucket_num],
			Marker:  &marker,
			MaxKeys: &max_keys,
		})
		end := time.Now().UnixNano()

		processAWSError(err)
		if err != nil {
			break
		}
		stats.addOp(thread_num, 0, end-start)
		stats.updateIntervals(thread_num)

		if *p.IsTruncated {
			if p.NextMarker != nil {
				marker = *p.NextMarker
			} else {
				// Use last object as marker if S3 service does not return the marker
				marker = *p.Contents[len(p.Contents)-1].Key
			}
		} else {
			marker = ""
			bucket_num = rand.Int63() % bucket_count
		}
	}
	stats.finish(thread_num)
	atomic.AddInt64(&running_threads, -1)
}

var cfg *aws.Config

func runBucketsInit(thread_num int, stats *Stats) {
	svc := s3.New(session.New(), cfg)

	for {
		bucket_num := atomic.AddInt64(&op_counter, 1)
		if bucket_num >= bucket_count {
			atomic.AddInt64(&op_counter, -1)
			break
		}
		start := time.Now().UnixNano()
		in := &s3.CreateBucketInput{Bucket: aws.String(buckets[bucket_num])}
		_, err := svc.CreateBucket(in)
		end := time.Now().UnixNano()
		stats.updateIntervals(thread_num)

		processAWSError(err)
		if err != nil {
			if !strings.Contains(err.Error(), s3.ErrCodeBucketAlreadyOwnedByYou) &&
				!strings.Contains(err.Error(), "BucketAlreadyExists") {
				log.Fatalf("FATAL: Unable to create bucket %s (is your access and secret correct?): %v", buckets[bucket_num], err)
			}
		}
		stats.addOp(thread_num, 0, end-start)
	}
	stats.finish(thread_num)
	atomic.AddInt64(&running_threads, -1)
}

type pagedObject struct {
	bucket_num int64
	key        string
	size       int64
}

func runPagedList(wg *sync.WaitGroup, bucket_num int64, list chan<- pagedObject) {
	svcL := GetS3Services("")

	svcL[0].ListObjectsPages(
		&s3.ListObjectsInput{
			Bucket:  &buckets[bucket_num],
			MaxKeys: &max_keys,
		},
		func(page *s3.ListObjectsOutput, last bool) bool {
			for _, v := range page.Contents {
				list <- pagedObject{
					bucket_num: bucket_num,
					key:        *v.Key,
					size:       *v.Size,
				}
			}
			return true
		})
	wg.Done()
}

func runBucketsClear(list <-chan pagedObject, thread_num int, stats *Stats) {
	iterator := int64(-1)
	svcL := GetS3Services("")

	for {
		iterator++

		v := <-list
		start := time.Now().UnixNano()
		_, err := svcL[iterator%int64(len(svcL))].DeleteObject(&s3.DeleteObjectInput{
			Bucket: &buckets[v.bucket_num],
			Key:    &v.key,
		})
		end := time.Now().UnixNano()
		stats.updateIntervals(thread_num)
		processAWSError(err)
		if err != nil {
			break
		}
		stats.addOp(thread_num, v.size, end-start)
	}
	stats.finish(thread_num)
	atomic.AddInt64(&running_threads, -1)
}

func runWrapper(loop int, r rune) []OutputStats {
	op_counter = first_object - 1
	running_threads = int64(threads)
	intervalNano := int64(interval * 1000000000)
	endtime = time.Now().Add(time.Second * time.Duration(duration_secs))
	var stats Stats

	// If we perviously set the object count after running a put
	// test, set the object count back to -1 for the new put test.
	if r == 'p' && object_count_flag {
		object_count = -1
		object_count_flag = false
	}

	switch r {
	case 'c':
		log.Printf("Running Loop %d BUCKET CLEAR TEST", loop)
		stats = makeStats(loop, "BCLR", threads, intervalNano)
		list := make(chan pagedObject, threads*2)
		var wg = sync.WaitGroup{}
		for b := int64(0); b < bucket_count; b++ {
			wg.Add(1)
			go runPagedList(&wg, b, list)
		}
		for n := 0; n < threads; n++ {
			go runBucketsClear(list, n, &stats)
		}
		wg.Wait()
		close(list)
	case 'x':
		log.Printf("Running Loop %d BUCKET DELETE TEST", loop)
		stats = makeStats(loop, "BDEL", threads, intervalNano)
		for n := 0; n < threads; n++ {
			go runBucketDelete(n, &stats)
		}
	case 'i':
		log.Printf("Running Loop %d BUCKET INIT TEST", loop)
		stats = makeStats(loop, "BINIT", threads, intervalNano)
		for n := 0; n < threads; n++ {
			go runBucketsInit(n, &stats)
		}
	case 'p':
		log.Printf("Running Loop %d OBJECT PUT TEST", loop)
		stats = makeStats(loop, "PUT", threads, intervalNano)
		for n := 0; n < threads; n++ {
			go runUpload(n, endtime, &stats)
		}
	case 'l':
		log.Printf("Running Loop %d BUCKET LIST TEST", loop)
		stats = makeStats(loop, "LIST", threads, intervalNano)
		for n := 0; n < threads; n++ {
			go runBucketList(n, &stats)
		}
	case 'g':
		log.Printf("Running Loop %d OBJECT GET TEST", loop)
		stats = makeStats(loop, "GET", threads, intervalNano)
		for n := 0; n < threads; n++ {
			go runDownload(n, endtime, &stats)
		}
	case 'd':
		log.Printf("Running Loop %d OBJECT DELETE TEST", loop)
		stats = makeStats(loop, "DEL", threads, intervalNano)
		for n := 0; n < threads; n++ {
			go runDelete(n, &stats)
		}
	}

	// Wait for it to finish
	for atomic.LoadInt64(&running_threads) > 0 {
		time.Sleep(time.Millisecond)
	}

	// If the user didn't set the object_count, we can set it here
	// to limit subsequent get/del tests to valid objects only.
	if r == 'p' && object_count < 0 {
		object_count = op_counter + 1
		object_count_flag = true
	}

	// Create the Output Stats
	os := make([]OutputStats, 0)
	for i := int64(0); i >= 0; i++ {
		if o, ok := stats.makeOutputStats(i); ok {
			os = append(os, o)
		} else {
			break
		}
	}
	if o, ok := stats.makeTotalStats(); ok {
		o.log()
		os = append(os, o)
	}
	return os
}

func init() {
	// Parse command line
	myflag := flag.NewFlagSet("myflag", flag.ExitOnError)
	myflag.StringVar(&access_key, "a", os.Getenv("AWS_ACCESS_KEY_ID"), "Access key")
	myflag.StringVar(&secret_key, "s", os.Getenv("AWS_SECRET_ACCESS_KEY"), "Secret key")
	myflag.StringVar(&url_host, "u", os.Getenv("AWS_HOST"), "URL for host with method prefix")
	myflag.StringVar(&object_prefix, "op", "", "Prefix for objects")
	myflag.StringVar(&bucket_prefix, "bp", "hotsauce-bench", "Prefix for buckets")
	myflag.StringVar(&bucket_list, "bl", "", "Use space-separated list of buckets for testing, not <prefix>000000000000")
	myflag.StringVar(&region, "r", "us-east-1", "Region for testing")
	myflag.StringVar(&storage_class, "cl", "", "Storage class to use")
	myflag.StringVar(&modes, "m", "cxiplgdcx", "Run modes in order.  See NOTES for more info")
	myflag.StringVar(&output, "o", "", "Write CSV output to this file")
	myflag.StringVar(&json_output, "j", "", "Write JSON output to this file")
	myflag.StringVar(&objects_info_output, "oj", "", "Detailed log for object operations")
	myflag.Int64Var(&max_keys, "mk", 1000, "Maximum number of keys to retreive at once for bucket listings")
	myflag.Int64Var(&object_count, "n", -1, "Maximum number of objects <-1 for unlimited>")
	myflag.Int64Var(&first_object, "f", 0, "Object number to start with")
	myflag.Int64Var(&bucket_count, "b", 1, "Number of buckets to distribute IOs across")
	myflag.IntVar(&duration_secs, "d", 60, "Maximum test duration in seconds <-1 for unlimited>")
	myflag.IntVar(&threads, "t", 1, "Number of threads to run")
	myflag.IntVar(&loops, "l", 1, "Number of times to repeat test")
	myflag.Int64Var(&op_timeout, "tt", 0, "Timeout for GET/PUT operations (in ms)")
	myflag.Int64Var(&ranged_offset, "ro", 0, "GET Ranged request: offset (bytes)")
	myflag.Int64Var(&ranged_size, "rs", 0, "GET Ranged request: size (bytes)")
	myflag.StringVar(&sizeArg, "z", "1M", "Size of objects in bytes with postfix K, M, and G")
	myflag.StringVar(&minSizeArg, "mz", "", "Minimum size of objects in bytes with postfix K, M, and G")
	myflag.Float64Var(&interval, "ri", 1.0, "Number of seconds between report intervals")
	myflag.StringVar(&workload_profile_file, "wp", "", "Name of workload profile file")
	myflag.StringVar(&workload_profile_name, "p", "", "Name of workload profile (default: first one)")
	// define custom usage output with notes
	notes :=
		`
NOTES:
  - Valid mode types for the -m mode string are:
    c: clear all existing objects from buckets (requires lookups)
    x: delete buckets
    i: initialize buckets 
    p: put objects in buckets
    l: list objects in buckets
    g: get objects from buckets (randomly when object count is known, sequentally otherwise)
    d: delete objects from buckets 

    These modes are processed in-order and can be repeated, ie "ippgd" will
    initialize the buckets, put the objects, reput the objects, get the
    objects, and then delete the objects.  The repeat flag will repeat this
    whole process the specified number of times.

  - When performing bucket listings, many S3 storage systems limit the
    maximum number of keys returned to 1000 even if MaxKeys is set higher.
    hsbench will attempt to set MaxKeys to whatever value is passed via the 
    "mk" flag, but it's likely that any values above 1000 will be ignored.
`
	myflag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "\nUSAGE: %s [OPTIONS]\n\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "OPTIONS:\n")
		myflag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), notes)
	}

	if err := myflag.Parse(os.Args[1:]); err != nil {
		os.Exit(1)
	}

	// Check workload profile
	if workload_profile_file != "" {
		w, err := LoadWorkloadConfig(workload_profile_file)
		if err != nil {
			log.Printf("Error loading workload profile config file: %v", err)
			return
		}
		workload_config = w
	} else {
		// Generate default workload profile
		workload_config.AddWorkloadProfile("", 1, ranged_size, ranged_offset)
	}

	// Configure S3 profile
	if len(workload_config.S3Config) < 1 {
		workload_config.AddS3Config("default", []string{url_host}, access_key, secret_key)
	} else {
		// Fill empty fields with ENV values
		for i, d := range workload_config.S3Config {
			if d.AccessKey == "" {
				workload_config.S3Config[i].AccessKey = access_key
			}
			if d.SecretKey == "" {
				workload_config.S3Config[i].SecretKey = secret_key
			}
			if len(d.Endpoints) < 1 {
				workload_config.S3Config[i].Endpoints = []string{url_host}
			}
		}
	}
	s3Config := workload_config.S3Config[0]
	cfg = &aws.Config{
		Endpoint:    aws.String(s3Config.Endpoints[0]),
		Credentials: credentials.NewStaticCredentials(s3Config.AccessKey, s3Config.SecretKey, ""),
		Region:      aws.String(region),
		// DisableParamValidation:  aws.Bool(true),
		DisableComputeChecksums: aws.Bool(true),
		S3ForcePathStyle:        aws.Bool(true),
	}

	// Check the arguments
	if object_count < 0 && duration_secs < 0 {
		log.Fatal("The number of objects and duration can not both be unlimited")
	}
	if s3Config.AccessKey == "" {
		log.Fatal("Missing argument -a for access key.")
	}
	if s3Config.SecretKey == "" {
		log.Fatal("Missing argument -s for secret key.")
	}
	if s3Config.Endpoints[0] == "" {
		log.Fatal("Missing argument -u for host endpoint.")
	}
	invalid_mode := false
	for _, r := range modes {
		if r != 'i' &&
			r != 'c' &&
			r != 'p' &&
			r != 'g' &&
			r != 'l' &&
			r != 'd' &&
			r != 'x' {
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
	var minSize uint64
	if size, err = bytefmt.ToBytes(sizeArg); err != nil {
		log.Fatalf("Invalid -z argument for object size: %v", err)
	}
	if len(minSizeArg) > 0 {
		if minSize, err = bytefmt.ToBytes(minSizeArg); err != nil {
			log.Fatalf("Invalid -z argument for object size: %v", err)
		}
	}
	object_max_size = int64(size)
	object_min_size = int64(minSize)
}

func generateSeed(key string, ts uint64) int64 {
	byte_ts := make([]byte, 8)
	binary.LittleEndian.PutUint64(byte_ts, ts)
	h := fnv.New64a()
	h.Write([]byte(key))
	h.Write(byte_ts)
	return int64(h.Sum64())
}

func main() {
	// Hello
	log.Printf("Hotsauce S3 Benchmark Version 0.x DEV")

	app_context = context.TODO()

	// Echo the parameters
	log.Printf("Parameters:")
	log.Printf("Workload config:", workload_config)
	log.Printf("url=%s", url_host)
	log.Printf("object_prefix=%s", object_prefix)
	if bucket_list != "" {
		log.Printf("bucket_list=%s", bucket_list)
	} else {
		log.Printf("bucket_prefix=%s", bucket_prefix)
	}
	log.Printf("region=%s", region)
	log.Printf("modes=%s", modes)
	log.Printf("output=%s", output)
	log.Printf("json_output=%s", json_output)
	log.Printf("objects_info_output=%s", objects_info_output)
	log.Printf("max_keys=%d", max_keys)
	log.Printf("object_count=%d", object_count)
	log.Printf("first_object=%d", first_object)
	log.Printf("bucket_count=%d", bucket_count)
	log.Printf("duration=%d", duration_secs)
	log.Printf("threads=%d", threads)
	log.Printf("loops=%d", loops)
	log.Printf("size=%s", sizeArg)
	log.Printf("min_size=%s", minSizeArg)
	log.Printf("interval=%f", interval)
	log.Printf("operation_timeout=%d", op_timeout)
	log.Printf("ranged_offset=%d", ranged_offset)
	log.Printf("ranged_size=%d", ranged_size)
	log.Printf("workload_profile_file=%s", workload_profile_file)
	log.Printf("workload_profile_name=%s", workload_profile_name)

	// Setup the slice of buckets
	if bucket_list == "" {
		for i := int64(0); i < bucket_count; i++ {
			buckets = append(buckets, fmt.Sprintf("%s%012d", bucket_prefix, i))
		}
	} else {
		buckets = strings.Split(bucket_list, " ")
	}

	response_statuses = make(map[string]int, 50)

	// Setup map of objects info
	object_info_chan = make(chan ObjectInfo, 1000)

	// Write objects info
	wg := sync.WaitGroup{}
	wg.Add(1)
	go LogObjectInfo(objects_info_output, object_info_chan, &wg)

	// Loop running the tests
	oStats := make([]OutputStats, 0)
	for loop := 0; loop < loops; loop++ {
		for _, r := range modes {
			oStats = append(oStats, runWrapper(loop, r)...)
		}
	}

	close(object_info_chan)
	wg.Wait()

	// Write CSV Output
	LogCSV(output, oStats)

	// Write JSON output
	LogJSON(json_output, oStats)
}
