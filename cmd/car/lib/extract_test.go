package lib

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/multiformats/go-multihash"
)

func rawRootLinkSystem(t *testing.T, content []byte) (cid.Cid, ipld.LinkSystem) {
	t.Helper()
	root, err := (cid.Prefix{
		Version:  1,
		Codec:    cid.Raw,
		MhType:   multihash.SHA2_256,
		MhLength: -1,
	}).Sum(content)
	if err != nil {
		t.Fatal(err)
	}
	ls := cidlink.DefaultLinkSystem()
	ls.StorageReadOpener = func(_ ipld.LinkContext, _ ipld.Link) (io.Reader, error) {
		return bytes.NewReader(content), nil
	}
	return root, ls
}

func TestExtractRawRoot(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content []byte
	}{
		{"binary", []byte{0, 255, 1, 10, 128}},
		{"empty", []byte{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, ls := rawRootLinkSystem(t, tc.content)
			dir := t.TempDir()
			var log bytes.Buffer
			count, err := ExtractToDir(context.Background(), &ls, root, dir, nil, false, &log)
			if err != nil || count != 1 {
				t.Fatalf("ExtractToDir() = (%d, %v), want (1, nil)", count, err)
			}
			got, err := os.ReadFile(filepath.Join(dir, root.String()))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, tc.content) {
				t.Fatalf("extracted bytes = %x, want %x", got, tc.content)
			}
			if !strings.Contains(log.String(), "not wrapped in a UnixFS directory") || !strings.Contains(log.String(), root.String()) {
				t.Fatalf("missing raw-root fallback warning: %q", log.String())
			}
		})
	}
}

func TestExtractRawRootReadError(t *testing.T) {
	root, ls := rawRootLinkSystem(t, []byte("missing"))
	readErr := errors.New("block unavailable")
	ls.StorageReadOpener = func(_ ipld.LinkContext, _ ipld.Link) (io.Reader, error) {
		return nil, readErr
	}
	dir := t.TempDir()
	var log bytes.Buffer
	count, err := ExtractToDir(context.Background(), &ls, root, dir, nil, false, &log)
	if count != 0 || !errors.Is(err, readErr) {
		t.Fatalf("ExtractToDir() = (%d, %v), want (0, %v)", count, err, readErr)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 || log.Len() != 0 {
		t.Fatalf("failed extraction left %d files and logged %q", len(entries), log.String())
	}
}

func TestExtractRawRootOutputError(t *testing.T) {
	root, ls := rawRootLinkSystem(t, []byte("content"))
	var log bytes.Buffer
	count, err := ExtractToDir(context.Background(), &ls, root, filepath.Join(t.TempDir(), "missing"), nil, false, &log)
	if count != 0 || err == nil {
		t.Fatalf("ExtractToDir() = (%d, %v), want an output error and zero files", count, err)
	}
	if log.Len() != 0 {
		t.Fatalf("failed extraction reported success: %q", log.String())
	}
}
