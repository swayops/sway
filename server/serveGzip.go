package server

import (
	"compress/gzip"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"mime"

	"github.com/gin-gonic/gin"
)

const (
	vary            = "Vary"
	acceptEncoding  = "Accept-Encoding"
	contentEncoding = "Content-Encoding"
)

func staticGzipServe(dir string) func(c *gin.Context) {
	serveAndAbort := func(c *gin.Context, fp string) {
		if filepath.Ext(fp) == ".gz" {
			c.Header("Content-Encoding", "gzip")
		}
		c.File(fp)
	}
	return func(c *gin.Context) {
		supportsGzip := strings.Contains(c.Request.Header.Get(acceptEncoding), "gzip")
		fp := filepath.Join(dir, c.Param("fp"))
		ext := filepath.Ext(fp)
		c.Header("Vary", "Accept-Encoding")
		c.Header("Content-Type", mime.TypeByExtension(ext))
		if !supportsGzip || ext == ".gz" {
			serveAndAbort(c, fp)
			return
		}
		if gzfp := fp + ".gz"; fileExists(gzfp) {
			serveAndAbort(c, gzfp)
			return
		}
		gzw := gzipWriterPool.Get().(*gzip.Writer)
		gzw.Reset(c.Writer)
		defer func() {
			gzw.Close()
			gzipWriterPool.Put(gzw)
		}()
		http.ServeFile(gzipResponseWriter{gzw, c.Writer}, c.Request, fp)
	}
}

func fileExists(fp string) bool {
	st, err := os.Stat(fp)
	return err == nil && !st.IsDir()
}

// from https://github.com/NYTimes/gziphandler/blob/master/gzip.go

var gzipWriterPool = sync.Pool{
	New: func() interface{} { return gzip.NewWriter(nil) },
}

// gzipResponseWriter provides an http.ResponseWriter interface, which gzips
// bytes before writing them to the underlying response. This doesn't set the
// Content-Encoding header, nor close the writers, so don't forget to do that.
type gzipResponseWriter struct {
	gw *gzip.Writer
	http.ResponseWriter
}

// Write appends data to the gzip writer.
func (w gzipResponseWriter) Write(b []byte) (int, error) {
	if _, ok := w.Header()["Content-Encoding"]; !ok {
		w.Header().Set("Content-Encoding", "gzip")
	}
	return w.gw.Write(b)
}

// Flush flushes the underlying *gzip.Writer and then the underlying
// http.ResponseWriter if it is an http.Flusher. This makes gzipResponseWriter
// an http.Flusher.
func (w gzipResponseWriter) Flush() {
	w.gw.Flush()
	if fw, ok := w.ResponseWriter.(http.Flusher); ok {
		fw.Flush()
	}
}
