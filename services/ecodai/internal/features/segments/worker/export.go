package worker

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
)

type ExportedObject struct {
	URI   string
	Bytes int64
}
type Exporter interface {
	Upload(context.Context, string, string) (ExportedObject, error)
}

func objectKey(id string) string { return "segments-v0/" + id + "/members.csv" }

type LocalExporter struct{ Root string }

func (e LocalExporter) Upload(ctx context.Context, id, source string) (ExportedObject, error) {
	if e.Root == "" {
		return ExportedObject{}, fmt.Errorf("local export root is empty")
	}
	final, err := filepath.Abs(filepath.Join(e.Root, filepath.FromSlash(objectKey(id))))
	if err != nil {
		return ExportedObject{}, err
	}
	if err := os.MkdirAll(filepath.Dir(final), 0700); err != nil {
		return ExportedObject{}, err
	}
	src, err := os.Open(source)
	if err != nil {
		return ExportedObject{}, err
	}
	defer src.Close()
	tmp, err := os.CreateTemp(filepath.Dir(final), "members-*.tmp")
	if err != nil {
		return ExportedObject{}, err
	}
	defer os.Remove(tmp.Name())
	n, err := io.Copy(tmp, &contextReader{ctx: ctx, Reader: src})
	if err != nil {
		tmp.Close()
		return ExportedObject{}, err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return ExportedObject{}, err
	}
	if err = tmp.Close(); err != nil {
		return ExportedObject{}, err
	}
	if err = os.Rename(tmp.Name(), final); err != nil {
		return ExportedObject{}, err
	}
	info, err := os.Stat(final)
	if err != nil {
		return ExportedObject{}, err
	}
	if info.Size() != n {
		return ExportedObject{}, fmt.Errorf("local export size mismatch")
	}
	return ExportedObject{URI: "file://" + final, Bytes: n}, nil
}

type contextReader struct {
	ctx context.Context
	io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(p)
}

type GCSExporter struct {
	Client *storage.Client
	Bucket string
}

func (e GCSExporter) Upload(ctx context.Context, id, source string) (ExportedObject, error) {
	if e.Client == nil || e.Bucket == "" {
		return ExportedObject{}, fmt.Errorf("GCS export is not configured")
	}
	f, err := os.Open(source)
	if err != nil {
		return ExportedObject{}, err
	}
	defer f.Close()
	key := objectKey(id)
	obj := e.Client.Bucket(e.Bucket).Object(key)
	w := obj.NewWriter(ctx)
	w.ContentType = "text/csv"
	w.CacheControl = "private, no-store"
	n, err := io.Copy(w, &contextReader{ctx: ctx, Reader: f})
	if err != nil {
		w.Close()
		return ExportedObject{}, err
	}
	if err = w.Close(); err != nil {
		return ExportedObject{}, err
	}
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return ExportedObject{}, err
	}
	if attrs.Size != n {
		return ExportedObject{}, fmt.Errorf("GCS object size mismatch")
	}
	return ExportedObject{URI: "https://storage.googleapis.com/" + url.PathEscape(e.Bucket) + "/" + strings.ReplaceAll(url.PathEscape(key), "%2F", "/"), Bytes: n}, nil
}
