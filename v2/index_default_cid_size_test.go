package car_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/ipfs/go-cid"
	carv2 "github.com/ipld/go-car/v2"
	"github.com/ipld/go-car/v2/index"
	"github.com/ipld/go-car/v2/internal/carv1"
	"github.com/ipld/go-car/v2/internal/carv1/util"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/sha3"
)

func TestIndexDefaultMaxCidSize(t *testing.T) {
	for _, delta := range []int{-1, 0, 1} {
		cidSize := int(carv2.DefaultMaxIndexCidSize) + delta
		t.Run(fmt.Sprintf("cid-size-%d", cidSize), func(t *testing.T) {
			data := []byte("default CID size limit")
			// SHAKE supports variable-length digests. Unlike identity CIDs,
			// these CIDs are indexed without enabling any options.
			digest := make([]byte, cidSize)
			mh, err := multihash.Encode(digest, multihash.SHAKE_256)
			require.NoError(t, err)
			overhead := cid.NewCidV1(cid.Raw, mh).ByteLen() - len(digest)
			digest = make([]byte, cidSize-overhead)
			sha3.ShakeSum256(digest, data)
			mh, err = multihash.Encode(digest, multihash.SHAKE_256)
			require.NoError(t, err)
			blockCid := cid.NewCidV1(cid.Raw, mh)
			require.Equal(t, cidSize, blockCid.ByteLen())

			var v1 bytes.Buffer
			err = carv1.WriteHeader(&carv1.CarHeader{Version: 1, Roots: []cid.Cid{blockCid}}, &v1)
			require.NoError(t, err)
			blockOffset := uint64(v1.Len())
			require.NoError(t, util.LdWrite(&v1, blockCid.Bytes(), data))

			// Build a CARv2 without an embedded index directly, so fixture
			// construction does not call the index-generation code under test.
			var v2 bytes.Buffer
			header := carv2.NewHeader(uint64(v1.Len()))
			header.IndexOffset = 0
			_, err = v2.Write(carv2.Pragma)
			require.NoError(t, err)
			_, err = header.WriteTo(&v2)
			require.NoError(t, err)
			_, err = v2.Write(v1.Bytes())
			require.NoError(t, err)

			for _, format := range []struct {
				name string
				data []byte
			}{
				{"CARv1", v1.Bytes()},
				{"CARv2", v2.Bytes()},
			} {
				for _, api := range []string{"GenerateIndex", "LoadIndex"} {
					t.Run(format.name+"/"+api, func(t *testing.T) {
						var got index.Index
						var err error
						if api == "GenerateIndex" {
							got, err = carv2.GenerateIndex(bytes.NewReader(format.data))
						} else {
							got, err = index.New(multicodec.CarMultihashIndexSorted)
							require.NoError(t, err)
							err = carv2.LoadIndex(got, bytes.NewReader(format.data))
						}
						if delta > 0 {
							var sizeErr *carv2.ErrCidTooLarge
							require.ErrorAs(t, err, &sizeErr)
							require.Equal(t, &carv2.ErrCidTooLarge{
								MaxSize:     carv2.DefaultMaxIndexCidSize,
								CurrentSize: uint64(cidSize),
							}, sizeErr)
							return
						}
						require.NoError(t, err)
						var offsets []uint64
						err = got.GetAll(blockCid, func(offset uint64) bool {
							offsets = append(offsets, offset)
							return true
						})
						require.NoError(t, err)
						require.Equal(t, []uint64{blockOffset}, offsets)
					})
				}
			}
		})
	}
}
