package main

import (
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"bytes"
	"bufio"
	"strings"
	"time"
	"io"
	"github.com/mailgun/groupcache/v2"
	"github.com/spf13/viper"
)



func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}


type ProxyHandler struct {
	stripHeaders []string
}


func (p ProxyHandler) GetKey(r http.Request) string {
    // Ensure we don't modify the headers on the original request
	copiedHeaders := make(http.Header, len(r.Header))
    for k, v := range r.Header {
        copiedHeaders[k] = append([]string(nil), v...)
    }

    r.Header = copiedHeaders
    for _, header := range p.stripHeaders {
        r.Header.Del(header)
    }

	key, err := httputil.DumpRequest(&r, true)
	if err != nil {
		panic(err)
	}

	return string(key)
}

func (p ProxyHandler) Handle (w http.ResponseWriter, r *http.Request) {
	// Serialize request to string (cache key)
	key := p.GetKey(*r)

	// Hit the cache
	var data []byte
	if err := group.Get(context.WithValue(r.Context(), "request", r), key, groupcache.AllocatingByteSliceSink(&data)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Serialize bytes into http response
	buf := bytes.NewBuffer(data)
	reader := bufio.NewReader(buf)
	res, err := http.ReadResponse(reader, r)
	if err != nil {
		log.Fatal(err)
	}

	copyHeader(w.Header(), res.Header)
	w.WriteHeader(res.StatusCode)
	io.Copy(w, res.Body)
	res.Body.Close()
}

func newPool(peers []string) *groupcache.HTTPPool {
	pool := groupcache.NewHTTPPoolOpts(peers[0], nil)
	pool.Set(peers...)

	return pool
}

var group *groupcache.Group

func newGroup(cacheSizeBytes int64) {
	group = groupcache.NewGroup("requests", cacheSizeBytes, groupcache.GetterFunc(
		func(ctx context.Context, key string, sink groupcache.Sink) error {
			me, err := os.Hostname()
			if err != nil {
				log.Panic(err)
			}

			log.Printf("Request handled by %s", me)

			// Get original request from context
			r, ok := ctx.Value("request").(*http.Request)
			if !ok {
				log.Panic("Can't get original request from context")
			}

			// We can't have this set on client requests
			r.RequestURI = ""
			// Server requests only have the path in the URL, but client requests need the scheme + host too
			r.URL, err= url.Parse("https://" + r.Host + r.URL.Path)
			if err != nil {
				log.Panic(err)
			}

			client := http.Client{}
 			res, err := client.Do(r)
			if err != nil {
				log.Panic(err)
			}

			// Write HTTP response out to bytes, store in cache
			var outBuf bytes.Buffer
			if err := res.Write(&outBuf); err != nil {
				log.Panic(err)
			}
			sink.SetBytes(outBuf.Bytes(), time.Time{})
			return nil
		},
	))
}


func main() {
	config := viper.New()
	config.AutomaticEnv()

	// Default to 50mb cache
	config.SetDefault("GROUPCACHE_SIZE_BYTES", 50000000)

	stripHeaders := config.GetString("STRIP_HEADERS")
	cachePeers := config.GetString("GROUPCACHE_PEERS")
	cacheSizeBytes := config.GetInt64("GROUPCACHE_SIZE_BYTES")
	if cachePeers == "" {
		log.Fatal("Missing required env variable GROUPCACHE_PEERS")
	}

	// Setup groupcache
	newGroup(cacheSizeBytes)
	peersList := strings.Split(cachePeers, ",")

	log.Printf("listening on %v", peersList[0])
	log.Printf("peers: %v", peersList)

	p := ProxyHandler{
		stripHeaders: strings.Split(stripHeaders, ","),
	} 
	http.HandleFunc("/", http.HandlerFunc(p.Handle))
	http.Handle("/_groupcache/", newPool(peersList))

	log.Fatal(http.ListenAndServe(":8080", nil))
}
