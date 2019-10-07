package nosql

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/globalsign/mgo"
)

type Mongo struct {
	db     *mgo.Database
	dbName string
}

var mongoSessions map[string]*mgo.Session
var lock = new(sync.Mutex)

func NewMongo(address, dbName string) (m *Mongo, err error) {
	if address == "" {
		return nil, errors.New("nil address")
	}
	dialInfo, err := mgo.ParseURL(address)
	if err != nil {
		return
	}
	if dbName == "" {
		dbName = dialInfo.Database
	}
	if dbName == "" {
		return nil, errors.New("nil dbName")
	}
	dialInfo.Timeout = 3 * time.Second

	var (
		mongoInstance *mgo.Session
		ok            bool
	)

	key := fmt.Sprintf("%s_%s", address, dbName)

	lock.Lock()
	defer lock.Unlock()
	if mongoInstance, ok = mongoSessions[key]; !ok {
		mongoInstance, err = mgo.DialWithInfo(dialInfo)
		if err != nil {
			return nil, fmt.Errorf("dial mongo fail: %#v", err)
		}
		mongoInstance.SetMode(mgo.Monotonic, true)
		mongoSessions[key] = mongoInstance
	}

	return &Mongo{
		db:     mongoInstance.Copy().DB(dbName),
		dbName: dbName,
	}, nil
}

func (m *Mongo) Close() {
	m.db.Session.Close()
}

func (m *Mongo) SetDbName(dbName string) {
	if dbName == m.dbName {
		return
	}
	m.db = m.db.Session.DB(dbName)
	m.dbName = dbName
}

func (m *Mongo) Exec(colName string, colFunc func(collection *mgo.Collection) error) error {
	return colFunc(m.db.C(colName))
}
