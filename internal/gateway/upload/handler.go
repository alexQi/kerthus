// Package upload bridges authenticated image uploads to private object storage.
package upload

import (
	"encoding/json"
	"fmt"
	me "go-micro.dev/v6/errors"
	"io"
	"kerthus/internal/platform/storage"
	"net/http"
	"net/url"
	"regexp"

	"strings"
	"time"
)

const maxSize int64 = 10 << 20

func answer(w http.ResponseWriter, data any, e error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	code, msg := int32(0), ""
	if e != nil {
		parsed := me.FromError(e)
		code = parsed.Code
		if code < 400 || code > 599 {
			code = 500
		}
		msg = parsed.Detail
		if code >= 500 {
			msg = "文件服务暂不可用"
		}
	}
	if code != 0 {
		w.WriteHeader(int(code))
	}
	json.NewEncoder(w).Encode(map[string]any{"code": code, "data": data, "msg": msg, "mesc": msg})
}

var publicKey = regexp.MustCompile(`^public/[1-9][0-9]*/[1-9][0-9]*/[a-f0-9]{64}\.(png|jpg|webp)$`)

func Files(store *storage.Store) http.Handler { return FilesWithLegacy(store, "") }

var legacyKey = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9/_\.-]{0,511}$`)

func FilesWithLegacy(store *storage.Store, legacyBase string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		key := strings.TrimPrefix(r.URL.Path, "/files/")
		if !publicKey.MatchString(key) {
			if !strings.HasPrefix(key, "private/") && legacyBase != "" && legacyKey.MatchString(key) && !strings.Contains(key, "..") {
				base, e := url.Parse(legacyBase)
				if e == nil && (base.Scheme == "https" || base.Scheme == "http") && base.Host != "" && base.User == nil {
					base.Path = strings.TrimRight(base.Path, "/") + "/" + key
					base.RawQuery = ""
					base.Fragment = ""
					http.Redirect(w, r, base.String(), http.StatusFound)
					return
				}
			}
			http.NotFound(w, r)
			return
		}
		body, size, ct, modified, e := store.Get(r.Context(), key)
		if e != nil {
			http.NotFound(w, r)
			return
		}
		defer body.Close()
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Content-Length", fmt.Sprint(size))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("Last-Modified", modified.UTC().Format(time.RFC1123))
		if r.Method == "GET" {
			_, _ = io.Copy(w, body)
		}
	})
}
