// Copyright (C) 2018 go-nebulas authors
//
// This file is part of the go-nebulas library.
//
// the go-nebulas library is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// the go-nebulas library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with the go-nebulas library.  If not, see <http://www.gnu.org/licenses/>.
//

package mvccdb

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nebulasio/go-nebulas/storage"
)

func TestMVCCDB_NewMVCCDB(t *testing.T) {
	storage, _ := storage.NewMemoryStorage()
	db, err := NewMVCCDB(storage)
	assert.Nil(t, err)

	assert.False(t, db.isInTransaction)
	assert.False(t, db.isPreparedDB)
}

func TestMVCCDB_FunctionEntryCondition(t *testing.T) {
	stor, _ := storage.NewMemoryStorage()
	db, _ := NewMVCCDB(stor)

	assert.Nil(t, db.Begin())
	assert.Equal(t, ErrUnsupportedNestedTransaction, db.Begin())
	assert.Nil(t, db.Commit())
	assert.Equal(t, ErrTransactionNotStarted, db.Commit())

	assert.Nil(t, db.Begin())
	assert.Nil(t, db.RollBack())
	assert.Equal(t, ErrTransactionNotStarted, db.RollBack())

	pdb, err := db.Prepare(nil)
	assert.Nil(t, pdb)
	assert.Error(t, ErrTransactionNotStarted, err)

	tid := "tid"
	assert.Nil(t, db.Begin())
	pdb, err = db.Prepare(nil)
	assert.Nil(t, pdb)
	assert.Equal(t, ErrTidIsNil, err)

	pdb, err = db.Prepare(tid)
	assert.NotNil(t, pdb)
	assert.Nil(t, err)

	deps, err := pdb.CheckAndUpdate()
	assert.Equal(t, 0, len(deps))
	assert.Nil(t, err)

	err = pdb.Reset()
	assert.Nil(t, err)
}

func TestMVCCDB_GetInTransaction(t *testing.T) {
	stor, _ := storage.NewMemoryStorage()
	db, _ := NewMVCCDB(stor)

	db.Begin()

	// get non-exist key.
	key := []byte("key")
	val, err := db.Get(key)
	assert.Nil(t, val)
	assert.Equal(t, storage.ErrKeyNotFound, err)

	// put to storage.
	stor.Put(key, []byte("value"))

	// get again.
	val, err = db.Get(key)
	assert.Equal(t, []byte("value"), val)
	assert.Nil(t, err)

	// put key1.
	key1 := []byte("key1")
	stor.Put(key1, []byte("value"))

	// get key1.
	val, err = db.Get(key1)
	assert.Equal(t, []byte("value"), val)
	assert.Nil(t, err)

}

func TestMVCCDB_DirectOpts(t *testing.T) {
	storage, _ := storage.NewMemoryStorage()
	db, _ := NewMVCCDB(storage)

	key := []byte("key")
	val := []byte("val")

	v, err := db.getFromStorage(key)
	assert.Nil(t, v)
	assert.NotNil(t, err)

	err = db.putToStorage(key, val)
	assert.Nil(t, err)

	v, err = db.getFromStorage(key)
	assert.Nil(t, err)
	assert.Equal(t, val, v)

	err = db.delFromStorage(key)
	assert.Nil(t, err)

	v, err = db.getFromStorage(key)
	assert.Nil(t, v)
	assert.NotNil(t, err)
}

func TestMVCCDB_OptsWithoutTransaction(t *testing.T) {
	storage, _ := storage.NewMemoryStorage()
	db, _ := NewMVCCDB(storage)

	key := []byte("key")
	val := []byte("val")

	v, err := db.Get(key)
	assert.Nil(t, v)
	assert.NotNil(t, err)

	err = db.Put(key, val)
	assert.Nil(t, err)

	v, err = db.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, val, v)

	err = db.Del(key)
	assert.Nil(t, err)

	v, err = db.Get(key)
	assert.Nil(t, v)
	assert.NotNil(t, err)
}

func TestMVCCDB_OptsWithinTransaction(t *testing.T) {
	store, _ := storage.NewMemoryStorage()
	db, _ := NewMVCCDB(store)

	key := []byte("key")
	val := []byte("val")

	err := db.Begin()
	assert.Nil(t, err)
	assert.True(t, db.isInTransaction)

	// unsupported nested transaction.
	err = db.Begin()
	assert.Equal(t, err, ErrUnsupportedNestedTransaction)

	v, err := db.Get(key)
	assert.Nil(t, v)
	assert.Equal(t, err, storage.ErrKeyNotFound)

	err = db.Put(key, val)
	assert.Nil(t, err)

	{
		// other MVCCDB can't read before commit.
		db2, _ := NewMVCCDB(store)
		v, err := db2.Get(key)
		assert.Nil(t, v)
		assert.Equal(t, err, storage.ErrKeyNotFound)
	}

	// commit.
	db.Commit()

	// read.
	v, err = db.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, val, v)

	// begin.
	err = db.Begin()
	assert.Nil(t, err)

	err = db.Del(key)
	assert.Nil(t, err)

	{
		// other MVCCDB read old value.
		db2, _ := NewMVCCDB(store)
		v, err := db2.Get(key)
		assert.Equal(t, val, v)
		assert.Nil(t, err)
	}

	v, err = db.Get(key)
	assert.Nil(t, v)
	assert.Equal(t, err, storage.ErrKeyNotFound)

	// rollback.
	db.RollBack()

	// read.
	v, err = db.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, val, v)

	{
		// other MVCCDB read old value.
		db2, _ := NewMVCCDB(store)
		v, err := db2.Get(key)
		assert.Equal(t, val, v)
		assert.Nil(t, err)
	}

	// begin.
	err = db.Begin()
	assert.Nil(t, err)

	err = db.Del(key)
	assert.Nil(t, err)

	// commit.
	db.Commit()

	// read.
	v, err = db.Get(key)
	assert.Nil(t, v)
	assert.Equal(t, err, storage.ErrKeyNotFound)

	{
		// other MVCCDB read nil.
		db2, _ := NewMVCCDB(store)
		v, err := db2.Get(key)
		assert.Nil(t, v)
		assert.Equal(t, err, storage.ErrKeyNotFound)
	}
}

func TestMVCCDB_PrepareAndUpdate(t *testing.T) {
	store, _ := storage.NewMemoryStorage()
	db, _ := NewMVCCDB(store)

	// init base data.
	db.Put([]byte("title"), []byte("this is test program"))

	// tid0 update.
	{
		db.Begin()

		tid := "tid0"
		pdb, err := db.Prepare(tid)
		assert.Nil(t, err)

		pdb.Put([]byte("duration"), []byte("65536"))
		pdb.Put([]byte("creator"), []byte("robin"))
		pdb.Put([]byte("count"), []byte("0"))
		pdb.Put([]byte("createdAt"), []byte("c0"))
		pdb.Put([]byte("updatedAt"), []byte("u0"))

		deps, err := pdb.CheckAndUpdate()
		assert.Nil(t, err)
		assert.Equal(t, 0, len(deps))

		db.Commit()
	}

	// concurrent update.
	db.Begin()

	type RetValue struct {
		tid     interface{}
		pdb     *MVCCDB
		depends []interface{}
		err     error
	}

	ret := make([]*RetValue, 0, 2)

	{
		tid := "tid1"
		pdb, err := db.Prepare(tid)
		assert.Nil(t, err)
		assert.NotNil(t, pdb)

		pdb.Get([]byte("count"))
		pdb.Put([]byte("count"), []byte("10"))
		pdb.Put([]byte("updatedAt"), []byte("u10"))

		ret = append(ret, &RetValue{
			tid: tid,
			pdb: pdb,
		})
	}

	{
		tid := "tid2"
		pdb, err := db.Prepare(tid)
		assert.Nil(t, err)
		assert.NotNil(t, pdb)

		pdb.Get([]byte("count"))
		pdb.Put([]byte("duration"), []byte("1024"))
		pdb.Put([]byte("updatedAt"), []byte("u20"))
		pdb.Del([]byte("creator"))
		pdb.Put([]byte("description"), []byte("new description"))

		ret = append(ret, &RetValue{
			tid: tid,
			pdb: pdb,
		})
	}

	for _, v := range ret {
		deps, err := v.pdb.CheckAndUpdate()
		v.depends = deps
		v.err = err
	}

	// commit.
	db.Commit()

	// verify.
	var finalRet, errorRet *RetValue
	for _, v := range ret {
		if v.err == nil {
			finalRet = v
		} else {
			errorRet = v
		}
	}

	assert.NotNil(t, finalRet)
	assert.NotNil(t, errorRet)

	assert.Nil(t, finalRet.err)
	assert.Equal(t, ErrStagingTableKeyConfliction, errorRet.err)

	assert.Equal(t, 1, len(finalRet.depends))
	assert.Equal(t, "tid0", finalRet.depends[0])
	assert.Equal(t, 0, len(errorRet.depends))

	// verify value.
	val, err := db.Get([]byte("title"))
	assert.Nil(t, err)
	assert.Equal(t, []byte("this is test program"), val)

	val, err = db.Get([]byte("duration"))
	assert.Nil(t, err)
	assert.Equal(t, []byte("65536"), val)

	val, err = db.Get([]byte("creator"))
	assert.Nil(t, err)
	assert.Equal(t, []byte("robin"), val)

	val, err = db.Get([]byte("count"))
	assert.Nil(t, err)
	assert.Equal(t, []byte("10"), val)

	val, err = db.Get([]byte("createdAt"))
	assert.Nil(t, err)
	assert.Equal(t, []byte("c0"), val)

	val, err = db.Get([]byte("updatedAt"))
	assert.Nil(t, err)
	assert.Equal(t, []byte("u10"), val)

	val, err = db.Get([]byte("description"))
	assert.Equal(t, storage.ErrKeyNotFound, err)
	assert.Nil(t, val)
}
