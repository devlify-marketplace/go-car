package car_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	carv2 "github.com/ipld/go-car/v2"
	"github.com/ipld/go-car/v2/index"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"
)

func TestReadOrGenerateIndexIdentityCIDs(t *testing.T) {
	data, err := os.ReadFile("testdata/sample-v1.car")
	require.NoError(t, err)

	for _, existing := range []struct {
		name         string
		fullyIndexed bool
	}{
		{"partial", false},
		{"full", true},
	} {
		t.Run(existing.name, func(t *testing.T) {
			var wrapped bytes.Buffer
			require.NoError(t, carv2.WrapV1(bytes.NewReader(data), &wrapped,
				carv2.StoreIdentityCIDs(existing.fullyIndexed)))

			// Mark the complete index explicitly: WrapV1 does not set this bit.
			reader, err := carv2.NewReader(bytes.NewReader(wrapped.Bytes()))
			require.NoError(t, err)
			header := reader.Header
			header.Characteristics.SetFullyIndexed(existing.fullyIndexed)
			var encodedHeader bytes.Buffer
			_, err = header.WriteTo(&encodedHeader)
			require.NoError(t, err)
			copy(wrapped.Bytes()[len(carv2.Pragma):], encodedHeader.Bytes())

			for _, requested := range []struct {
				name          string
				storeIdentity bool
			}{
				{"default", false},
				{"store-identity", true},
			} {
				t.Run(requested.name, func(t *testing.T) {
					opts := []carv2.Option{carv2.StoreIdentityCIDs(requested.storeIdentity)}
					got, err := carv2.ReadOrGenerateIndex(bytes.NewReader(wrapped.Bytes()), opts...)
					require.NoError(t, err)

					wantIdentity := existing.fullyIndexed || requested.storeIdentity
					want, err := carv2.GenerateIndex(bytes.NewReader(data), carv2.StoreIdentityCIDs(wantIdentity))
					require.NoError(t, err)
					require.Equal(t, want, got)

					blocks, err := carv2.NewBlockReader(bytes.NewReader(data))
					require.NoError(t, err)
					foundIdentity := false
					for {
						block, err := blocks.Next()
						if err == io.EOF {
							break
						}
						require.NoError(t, err)
						if block.Cid().Prefix().MhType == multihash.IDENTITY {
							foundIdentity = true
							_, err := index.GetFirst(got, block.Cid())
							if wantIdentity {
								require.NoError(t, err)
							} else {
								require.ErrorIs(t, err, index.ErrNotFound)
							}
						}
					}
					require.True(t, foundIdentity, "fixture must contain identity CIDs")

					// Compatible indexes must still be read, not regenerated. A tiny
					// generation limit also checks errors on the regeneration path.
					opts = append(opts, carv2.MaxIndexCidSize(1))
					limited, err := carv2.ReadOrGenerateIndex(bytes.NewReader(wrapped.Bytes()), opts...)
					if requested.storeIdentity && !existing.fullyIndexed {
						var tooLarge *carv2.ErrCidTooLarge
						require.ErrorAs(t, err, &tooLarge)
						require.Nil(t, limited)
					} else {
						require.NoError(t, err)
						require.Equal(t, got, limited)
					}
				})
			}
		})
	}
}
