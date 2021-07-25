// Copyright 2021 Trim21<trim21.me@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

// nolint
package main

import (
	"archive/zip"
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"sci_hub_p2p/pkg/dagserv"
	"sci_hub_p2p/pkg/logger"
	"sci_hub_p2p/pkg/vars"
)

func main() {
	LoadTestData()
}

func LoadTestData() {
	const count = 8
	bar := pb.StartNew(100000 - 4)
	zipFiles, err := filepath.Glob("d:/data/11200*/*.zip")
	if err != nil {
		logger.Fatal(err)
	}

	err = os.Remove("./test.bolt")
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			logger.Fatal(err)
		}
	}

	c := make(chan string, count)
	wg := sync.WaitGroup{}
	wg.Add(count)
	var dbSlice []*bbolt.DB
	for i := 0; i < count; i++ {
		db, err := bbolt.Open(fmt.Sprintf("./tmp/test-%d.bolt", i), 0600, &bbolt.Options{
			FreelistType: bbolt.FreelistMapType,
			NoSync:       true,
		})
		if err != nil {
			logger.Fatal(err)
		}
		dbSlice = append(dbSlice, db)
		db.Update(func(tx *bbolt.Tx) error {
			err := tx.DeleteBucket(vars.BlockBucketName())
			if err != nil {
				logger.Fatal(err)
			}
			err = tx.DeleteBucket(vars.NodeBucketName())
			if err != nil {
				logger.Fatal(err)
			}
			return nil
		})
		err = dagserv.InitDB(db)
		if err != nil {
			logger.Fatal(err)
		}
		go func(db *bbolt.DB) {
			for file := range c {
				err = func() error {
					r, err := zip.OpenReader(file)
					if err != nil {
						return err
					}
					defer r.Close()
					for _, f := range r.File {
						bar.Increment()
						offset, err := f.DataOffset()
						size := f.CompressedSize64
						r, err := f.Open()
						if err != nil {
							return err
						}
						_, err = dagserv.Add(db, r, file, int64(size), offset)
						if err != nil {
							_ = r.Close()
							return err
						}
						_ = r.Close()
					}
					return nil
				}()
				if err != nil {
					logger.Error(err)
				}
			}

			err := db.Sync()
			if err != nil {
				logger.Error(err)
			}
			wg.Done()
		}(db)
	}

	for _, file := range zipFiles {
		c <- file
	}

	for len(c) != 0 {
		time.Sleep(time.Second)
	}

	close(c)

	wg.Wait()
	bar.Finish()

	db, err := bbolt.Open("./test.bolt", 0600, &bbolt.Options{
		FreelistType: bbolt.FreelistMapType,
		NoSync:       true,
	})
	if err != nil {
		logger.Fatal(err)
	}
	err = dagserv.InitDB(db)
	if err != nil {
		logger.Fatal(err)
	}

	for i, srcDB := range dbSlice {
		fmt.Println("copy db", i)
		err = copyBucket(srcDB, db, vars.NodeBucketName())

		if err != nil {
			logger.Error(err)
		}

		err = copyBucket(srcDB, db, vars.BlockBucketName())

		if err != nil {
			logger.Error(err)
		}

		err = srcDB.Close()
		if err != nil {
			logger.Fatal(err)
		}

		err := db.Sync()
		if err != nil {
			logger.Fatal(err)
		}

	}

	err = db.Close()
	if err != nil {
		logger.Fatal(err)
	}
}

func copyBucket(src, dst *bbolt.DB, name []byte) error {
	return dst.Batch(func(dstTx *bbolt.Tx) error {
		dstBucket, err := dstTx.CreateBucketIfNotExists(name)
		if err != nil {
			return errors.Wrap(err, "failed to create bucket in dst DB")
		}

		return src.View(func(srcTx *bbolt.Tx) error {
			srcBucket := srcTx.Bucket(name)

			return srcBucket.ForEach(func(k, v []byte) error {
				if bytes.Equal(name, vars.NodeBucketName()) {
					_, err := cid.Parse(k)
					if err != nil {
						logger.Error(err)
						fmt.Println(hex.Dump(k))
					}
				}

				return dstBucket.Put(k, v)
			})
		})
	})
}