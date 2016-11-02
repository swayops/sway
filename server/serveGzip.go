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
	contentType     = "Content-Type"
	contentEncoding = "Content-Encoding"
)

func staticGzipServe(dir string) func(c *gin.Context) {
	acceptsGzip := func(c *gin.Context) bool {
		return strings.Contains(c.Request.Header.Get(acceptEncoding), "gzip")
	}
	return func(c *gin.Context) {
		p := c.Param("fp")
		if p == "" {
			p = c.Request.URL.Path[len("/static/"):]
		}

		var (
			fp   = filepath.Join(dir, p)
			ext  = strings.ToLower(filepath.Ext(fp))
			typ  = mime.TypeByExtension(ext)
			pass bool
		)

		c.Header(vary, acceptEncoding)
		c.Header(contentType, typ)

		switch ext {
		case ".jpg", ".png", ".pdf", ".gz", ".jpeg", ".gif", ".woff", ".ttf", ".eot", ".mp4":
			pass = true
		}

		if pass || !acceptsGzip(c) {
			c.File(fp)
			return
		}

		c.Header(contentEncoding, "gzip")

		if gzfp := fp + ".gz"; fileExists(gzfp) {
			c.File(gzfp)
			return
		}

		gzw := gzipWriterPool.Get().(*gzip.Writer)
		gzw.Reset(c.Writer)
		http.ServeFile(gzipResponseWriter{gzw, c.Writer}, c.Request, fp)
		gzw.Close()
		gzw.Reset(nil)
		gzipWriterPool.Put(gzw)
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
