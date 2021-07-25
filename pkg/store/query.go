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

package store

import (
	ds "github.com/ipfs/go-datastore"
	dsq "github.com/ipfs/go-datastore/query"
	dshelp "github.com/ipfs/go-ipfs-ds-help"
	"github.com/jbenet/goprocess"
	"github.com/sirupsen/logrus"
	"go.etcd.io/bbolt"

	"sci_hub_p2p/pkg/logger"
	"sci_hub_p2p/pkg/vars"
)

func queryBolt(d *MapDataStore, q dsq.Query, log *logrus.Entry) (dsq.Results, error) {
	// Special case order by key.
	orders := q.Orders
	if len(orders) > 0 {
		switch q.Orders[0].(type) {
		case dsq.OrderByKey, *dsq.OrderByKey:
			// Already ordered by key.
			orders = nil
		}
	}

	qrb := dsq.NewResultBuilder(q)
	qrb.Process.Go(func(worker goprocess.Process) {
		log.Debug("start process")
		defer log.Debug("stop process")

		err := d.db.View(func(tx *bbolt.Tx) error {
			buck := tx.Bucket(vars.BlockBucketName())
			c := buck.Cursor()

			// If we need to sort, we'll need to collect all the
			// results up-front.
			if len(orders) > 0 {
				// Query and filter.
				var entries []dsq.Entry
				for k, v := c.First(); k != nil; k, v = c.Next() {
					dk := MultiHashToKey(k).String()
					e := dsq.Entry{Key: dk}
					if !qrb.Query.KeysOnly {
						// We copy _after_ filtering/sorting.
						e.Value = v
					}
					if filter(q.Filters, e) {
						continue
					}
					entries = append(entries, e)
				}

				// sort
				dsq.Sort(orders, entries)

				// offset/limit
				entries = entries[qrb.Query.Offset:]
				if qrb.Query.Limit > 0 {
					if qrb.Query.Limit < len(entries) {
						entries = entries[:qrb.Query.Limit]
					}
				}

				// Send
				for _, e := range entries {
					// Copy late so we don't have to copy
					// values we don't use.
					e.Value = append(e.Value[0:0:0], e.Value...)
					select {
					case qrb.Output <- dsq.Result{Entry: e}:
					case <-worker.Closing(): // client told us to end early.
						return nil
					}
				}
			} else {
				// Otherwise, send results as we get them.
				offset := 0
				for k, v := c.First(); k != nil; k, v = c.Next() {
					dk := MultiHashToKey(k).String()
					e := dsq.Entry{Key: dk, Value: v}
					if !qrb.Query.KeysOnly {
						// We copy _after_ filtering.
						e.Value = v
					}

					// pre-filter
					if filter(q.Filters, e) {
						continue
					}

					// now count this item towards the results
					offset++

					// check the offset
					if offset < qrb.Query.Offset {
						continue
					}

					e.Value = append(e.Value[0:0:0], e.Value...)
					select {
					case qrb.Output <- dsq.Result{Entry: e}:
						offset++
					case <-worker.Closing():
						return nil
					}

					if qrb.Query.Limit > 0 &&
						offset >= (qrb.Query.Offset+qrb.Query.Limit) {
						// all done.
						return nil
					}
				}
			}

			return nil
		})
		if err != nil {
			log.Error(err)
		}
	})

	// go wait on the worker (without signaling close)

	go func() {
		err := qrb.Process.CloseAfterChildren()
		if err != nil {
			logger.Error(err)
		}
	}()

	return qrb.Results(), nil
}

func MultiHashToKey(k []byte) ds.Key {
	return topLevelBlockKey.Child(dshelp.MultihashToDsKey(k))
}

// filter checks if we should filter out the query.
func filter(filters []dsq.Filter, entry dsq.Entry) bool {
	for _, filter := range filters {
		if !filter.Filter(entry) {
			return true
		}
	}

	return false
}