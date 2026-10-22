package blockstore

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	carv2 "github.com/ipld/go-car/v2"
	"github.com/ipld/go-car/v2/index"
	"github.com/stretchr/testify/require"
)

func TestNewReadOnlyIgnoresReaderOffset(t *testing.T) {
	for _, fixture := range []string{
		"sample-v1.car",
		"sample-wrapped-v2.car",
		"sample-v2-indexless.car",
	} {
		t.Run(fixture, func(t *testing.T) {
			path := "../testdata/" + fixture
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			reader, err := carv2.NewBlockReader(bytes.NewReader(data))
			require.NoError(t, err)
			wantBlock, err := reader.Next()
			require.NoError(t, err)

			for _, backingType := range []string{"file", "bytes"} {
				t.Run(backingType, func(t *testing.T) {
					for _, indexMode := range []string{"generated", "supplied"} {
						t.Run(indexMode, func(t *testing.T) {
							for _, position := range []struct {
								name   string
								offset int64
								whence int
							}{
								{"start", 0, io.SeekStart},
								{"nonzero", 1, io.SeekStart},
								{"exhausted", 0, io.SeekEnd},
							} {
								t.Run(position.name, func(t *testing.T) {
									var backing interface {
										io.ReaderAt
										io.ReadSeeker
									}
									if backingType == "file" {
										f, err := os.Open(path)
										require.NoError(t, err)
										t.Cleanup(func() { require.NoError(t, f.Close()) })
										backing = f
									} else {
										backing = bytes.NewReader(data)
									}

									var idx index.Index
									if indexMode == "supplied" {
										idx, err = carv2.GenerateIndex(backing)
										require.NoError(t, err)
									}
									_, err := backing.Seek(position.offset, position.whence)
									require.NoError(t, err)

									subject, err := NewReadOnly(backing, idx)
									require.NoError(t, err)
									t.Cleanup(func() { require.NoError(t, subject.Close()) })
									roots, err := subject.Roots()
									require.NoError(t, err)
									require.Equal(t, reader.Roots, roots)
									gotBlock, err := subject.Get(context.Background(), wantBlock.Cid())
									require.NoError(t, err)
									require.Equal(t, wantBlock.RawData(), gotBlock.RawData())
								})
							}
						})
					}
				})
			}
		})
	}
}
