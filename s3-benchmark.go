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
	"math/rand"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/bytefmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// Global variables
var access_key, secret_key, url_host, bucket_prefix, region, sizeArg string
var buckets []string
var duration_secs, threads, loops, bucket_count int
var object_data []byte
var object_data_md5 string
var object_size uint64
var running_threads, object_count, upload_count, download_count, delete_count, upload_slowdown_count, download_slowdown_count, delete_slowdown_count int64
var endtime, upload_finish, download_finish, delete_finish time.Time

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

func createBucket(bucket_num int, ignore_errors bool) {
	svc := s3.New(session.New(), cfg)
	log.Printf(buckets[bucket_num])
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

func deleteAllObjects(bucket_num int) {
	svc := s3.New(session.New(), cfg)
	out, err := svc.ListObjects(&s3.ListObjectsInput{Bucket: &buckets[bucket_num]})
	if err != nil {
		log.Fatal("can't list objects")
	}
	n := len(out.Contents)
	if n == 0 {
		return
	}
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
	fmt.Printf("after delete, got %v objects\n", len(out.Contents))
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

func runUpload(thread_num int) {
//        bucket_num := thread_num % bucket_count
	errcnt := 0
	svc := s3.New(session.New(), cfg)
	for {
		if duration_secs > -1 && time.Now().After(endtime) {
			break
		}
                objnum := atomic.AddInt64(&upload_count, 1)
                bucket_num := objnum % int64(bucket_count)
		if object_count > -1 && objnum > object_count {
			objnum = atomic.AddInt64(&upload_count, -1)
			break
		}
		fileobj := bytes.NewReader(object_data)
		//prefix := fmt.Sprintf("%s/%s/Object-%d", url_host, buckets[bucket_num], objnum)

		key := fmt.Sprintf("Object-%d", objnum)
		r := &s3.PutObjectInput{
			Bucket: &buckets[bucket_num],
			Key:    &key,
			Body:   fileobj,
		}

		req, _ := svc.PutObjectRequest(r)
		// Disable payload checksum calculation (very expensive)
		req.HTTPRequest.Header.Add("X-Amz-Content-Sha256", "UNSIGNED-PAYLOAD")
		err := req.Send()
		if err != nil {
			errcnt++
			atomic.AddInt64(&upload_slowdown_count, 1)
			atomic.AddInt64(&upload_count, -1)
			fmt.Println("upload err", err)
			//break
		}
		if errcnt > 2 {
			break
		}
		fmt.Fprintf(os.Stderr, "upload thread %v, %v\r", thread_num, key)
	}
	// Remember last done time
	upload_finish = time.Now()
	// One less thread
	atomic.AddInt64(&running_threads, -1)
}

func runDownload(thread_num int) {
	errcnt := 0
	svc := s3.New(session.New(), cfg)
	for {
		if duration_secs > -1 && time.Now().After(endtime) {
                        break
                }

		objnum := atomic.AddInt64(&download_count, 1)
                if objnum > object_count {
			atomic.AddInt64(&download_count, -1)
                        break
                }

		bucket_num := objnum % int64(bucket_count)
                key := fmt.Sprintf("Object-%d", objnum)
                fmt.Fprintf(os.Stderr, "download thread %v, %v\r", thread_num, key)
                r := &s3.GetObjectInput{
                        Bucket: &buckets[bucket_num],
                        Key:    &key,
                }

                req, resp := svc.GetObjectRequest(r)
                err := req.Send()
                if err != nil {
                        errcnt++
                        atomic.AddInt64(&download_slowdown_count, 1)
                        atomic.AddInt64(&download_count, -1)
                        fmt.Println("download err", err)
                        //break
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
}

func runDelete(thread_num int) {
	errcnt := 0
	svc := s3.New(session.New(), cfg)

	for {
                objnum := atomic.AddInt64(&delete_count, 1)
		if objnum > object_count {
                        atomic.AddInt64(&delete_count, -1)
			break
		}

                bucket_num := objnum % int64(bucket_count)

                key := fmt.Sprintf("Object-%d", objnum)
                fmt.Fprintf(os.Stderr, "delete thread %v, %v\r", thread_num, key)
                r := &s3.DeleteObjectInput{
                        Bucket: &buckets[bucket_num],
                        Key:    &key,
                }

                req, out := svc.DeleteObjectRequest(r)
                err := req.Send()
                if err != nil {
                        errcnt++
                        atomic.AddInt64(&delete_slowdown_count, 1)
                        atomic.AddInt64(&delete_count, -1)
                        fmt.Println("delete err", err, "out", out.String())
                }
                if errcnt > 2 {
			break
                }
                fmt.Fprintf(os.Stderr, "delete thread %v, %v\r", thread_num, key)
	}
	// Remember last done time
	delete_finish = time.Now()
	// One less thread
	atomic.AddInt64(&running_threads, -1)
}

var cfg *aws.Config

func init() {
	// Parse command line
	myflag := flag.NewFlagSet("myflag", flag.ExitOnError)
	myflag.StringVar(&access_key, "a", os.Getenv("AWS_ACCESS_KEY_ID"), "Access key")
	myflag.StringVar(&secret_key, "s", os.Getenv("AWS_SECRET_ACCESS_KEY"), "Secret key")
	myflag.StringVar(&url_host, "u", os.Getenv("AWS_HOST"), "URL for host with method prefix")
	myflag.StringVar(&bucket_prefix, "p", "hotsauce_benchmark", "Prefix for buckets")
        myflag.IntVar(&bucket_count, "b", 1, "Number of buckets to distribute IOs across")
	myflag.StringVar(&region, "r", "us-east-1", "Region for testing")
        myflag.Int64Var(&object_count, "n", -1, "Maximum number of objects <-1 for unlimited>")
	myflag.IntVar(&duration_secs, "d", 60, "Maximum test duration in seconds <-1 for unlimited>")
	myflag.IntVar(&threads, "t", 1, "Number of threads to run")
	myflag.IntVar(&loops, "l", 1, "Number of times to repeat test")
	myflag.StringVar(&sizeArg, "z", "1M", "Size of objects in bytes with postfix K, M, and G")
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
	var err error
	if object_size, err = bytefmt.ToBytes(sizeArg); err != nil {
		log.Fatalf("Invalid -z argument for object size: %v", err)
	}
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

	// Initialize data for the bucket
	object_data = make([]byte, object_size)
	rand.Read(object_data)
	hasher := md5.New()
	hasher.Write(object_data)
	object_data_md5 = base64.StdEncoding.EncodeToString(hasher.Sum(nil))

	// Create the buckets and delete all the objects
        for i := 0; i < bucket_count; i++ {
                buckets = append(buckets, fmt.Sprintf("%s-%d", bucket_prefix, i))
		createBucket(i, true)
		deleteAllObjects(i)
        }

	var uploadspeed, downloadspeed float64

	// Loop running the tests
	for loop := 1; loop <= loops; loop++ {

		// reset counters
		upload_count = 0
		upload_slowdown_count = 0
		download_count = 0
		download_slowdown_count = 0
		delete_count = 0
		delete_slowdown_count = 0

		// Run the upload case
		running_threads = int64(threads)
		starttime := time.Now()
		endtime = starttime.Add(time.Second * time.Duration(duration_secs))
		for n := 0; n < threads; n++ {
			go runUpload(n)
		}

		// Wait for it to finish
		for atomic.LoadInt64(&running_threads) > 0 {
			time.Sleep(time.Millisecond)
		}
		upload_time := upload_finish.Sub(starttime).Seconds()

		bps := float64(uint64(upload_count)*object_size) / upload_time
		logit(fmt.Sprintf("Loop %d: PUT time %.1f secs, objects = %d, speed = %sB/sec, %.1f operations/sec. Slowdowns = %d",
			loop, upload_time, upload_count, bytefmt.ByteSize(uint64(bps)), float64(upload_count)/upload_time, upload_slowdown_count))

		uploadspeed = bps / bytefmt.MEGABYTE

		// Run the download case
		running_threads = int64(threads)
		starttime = time.Now()
		endtime = starttime.Add(time.Second * time.Duration(duration_secs))
		for n := 0; n < threads; n++ {
			go runDownload(n)
		}

		// Wait for it to finish
		for atomic.LoadInt64(&running_threads) > 0 {
			time.Sleep(time.Millisecond)
		}
		download_time := download_finish.Sub(starttime).Seconds()

		bps = float64(uint64(download_count)*object_size) / download_time
		logit(fmt.Sprintf("Loop %d: GET time %.1f secs, objects = %d, speed = %sB/sec, %.1f operations/sec. Slowdowns = %d",
			loop, download_time, download_count, bytefmt.ByteSize(uint64(bps)), float64(download_count)/download_time, download_slowdown_count))

		downloadspeed = bps / bytefmt.MEGABYTE

		// Run the delete case
		running_threads = int64(threads)
		starttime = time.Now()
		endtime = starttime.Add(time.Second * time.Duration(duration_secs))
		for n := 0; n < threads; n++ {
			go runDelete(n)
		}

		// Wait for it to finish
		for atomic.LoadInt64(&running_threads) > 0 {
			time.Sleep(time.Millisecond)
		}
		delete_time := delete_finish.Sub(starttime).Seconds()

		logit(fmt.Sprintf("Loop %d: DELETE time %.1f secs, %.1f deletes/sec. Slowdowns = %d",
			loop, delete_time, float64(upload_count)/delete_time, delete_slowdown_count))
	}

	// All done
	name := strings.Split(strings.TrimPrefix(url_host, "http://"), ".")[0]
	fmt.Printf("result title: name-concurrency-size, uloadspeed, downloadspeed\n")
	fmt.Printf("result csv: %v-%v-%v,%.2f,%.2f\n", name, threads, sizeArg, uploadspeed, downloadspeed)
}
