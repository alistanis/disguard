package disguard // import "go.zeta.pm/disguard"
import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

// WrappedReverseProxy wraps httputil reverse proxy handler to provide RequireSession
// Support so it would redirect to login.
type WrappedReverseProxy struct {
	*httputil.ReverseProxy

	sess *Session
}

// IsIgnoredPath checks whatever the path is ignored in the config
func (o *Session) isIgnoredPath(path string) bool {
	for _, c := range o.config.IgnoredPaths {
		if c == path {
			return true
		}
	}
	return false
}

func (w *WrappedReverseProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if w.sess.config.RequireSession {
		u, err := w.sess.getSession(req)
		if (err != nil || len(u.Whitelisted) == 0) && !w.sess.isIgnoredPath(req.URL.Path) {
			http.Redirect(rw, req, "/oauth/login", http.StatusFound)
			return
		}
	}
	w.ReverseProxy.ServeHTTP(rw, req)
}

// ReverseHandler ...
func (o *Session) ReverseHandler() *WrappedReverseProxy {
	target, err := url.Parse(o.config.ProxyAddress)
	if err != nil {
		log.Fatal(err)
	}
	targetQuery := target.RawQuery
	director := func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = singleJoiningSlash(target.Path, req.URL.Path)
		if targetQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = targetQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
		}
		if _, ok := req.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default value
			req.Header.Set("User-Agent", "")
		}
		if u, err := o.getSession(req); err == nil {
			req.Header.Set(o.config.HeaderName, strings.Join(u.Whitelisted, ","))
		} else {
			req.Header.Set(o.config.HeaderName, "")
		}
	}
	return &WrappedReverseProxy{&httputil.ReverseProxy{Director: director}, o}
}
