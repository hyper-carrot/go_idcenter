package provider

import (
	"errors"
	"fmt"
	"github.com/ziutek/mymysql/autorc"
	_ "github.com/ziutek/mymysql/thrsafe"
	. "go_idcenter/base"
	"go_idcenter/lib"
	"sync"
	"time"
)

const (
	TABLE_NAME = "group"
	TIMEOUT_MS = time.Duration(100)
)

type StorageParameter struct {
	Name     string
	Ip       string
	Port     int
	DbName   string
	User     string
	Password string
	PoolSize uint16
}

type mysqlStorageProvider struct {
	ProviderName string
}

var storageInitContext sync.Once
var mysqlConnPool *lib.Pool
var signMap map[string]*lib.Sign
var iMysqlStorageProvider *mysqlStorageProvider

func NewStorageProvider(parameter StorageParameter) *mysqlStorageProvider {
	storageInitContext.Do(func() {
		err := initializeForStorageProvider(parameter)
		if err != nil {
			panic(err)
		}
	})
	return iMysqlStorageProvider
}

func initializeForStorageProvider(parameter StorageParameter) error {
	mysqlServerAddr := fmt.Sprintf("%v:%v", parameter.Ip, parameter.Port)
	lib.LogInfof("Initialize mysql storage provider (parameter=%v)...", parameter)
	mysqlConnPool := &lib.Pool{Id: "MySQL Connection Pool", Size: int(parameter.PoolSize)}
	initFunc := func() (interface{}, error) {
		conn := autorc.New("tcp", "", mysqlServerAddr, parameter.User, parameter.Password)
		conn.Raw.Register("set names utf8")
		err := conn.Use(parameter.DbName)
		if err != nil {
			errorMsg := fmt.Sprintf("Occur error when mysql connection initialization (parameter=%v): %s\n", parameter, err)
			lib.LogErrorln(errorMsg)
			return nil, err
		}
		return conn, nil
	}
	err := mysqlConnPool.Init(initFunc)
	if err != nil {
		errorMsg := fmt.Sprintf("Occur error when mysql connection pool initialization (parameter=%v): %s\n", parameter, err)
		lib.LogErrorln(errorMsg)
		return errors.New(errorMsg)
	}
	signMap = make(map[string]*lib.Sign)
	iMysqlStorageProvider = &mysqlStorageProvider{parameter.Name}
	return nil
}

func getMysqlConnection() (*autorc.Conn, error) {
	element, ok := mysqlConnPool.Get(TIMEOUT_MS)
	if !ok {
		errorMsg := fmt.Sprintf("Getting mysql connection is FAILING!")
		return nil, errors.New(errorMsg)
	}
	if element == nil {
		errorMsg := fmt.Sprintf("The mysql connection is UNUSABLE!")
		return nil, errors.New(errorMsg)
	}
	var conn *autorc.Conn
	switch t := element.(type) {
	case *autorc.Conn:
		conn = element.(*autorc.Conn)
	default:
		errorMsg := fmt.Sprintf("The type of element in pool is UNMATCHED! (type=%v)", t)
		return nil, errors.New(errorMsg)
	}
	return conn, nil
}

func releaseMysqlConnection(conn *autorc.Conn) bool {
	if conn == nil {
		return false
	}
	result := mysqlConnPool.Put(conn, TIMEOUT_MS)
	return result
}

func (self *mysqlStorageProvider) BuildInfo(group string, start uint64, step uint32) (bool, error) {
	if len(group) == 0 {
		errorMsg := fmt.Sprint("The group name is INVALID!")
		lib.LogErrorln(errorMsg)
		return false, errors.New(errorMsg)
	}
	errorMsgPrefix := fmt.Sprintf("Occur error when build group info (group=%v, start=%v, step=%v)", group, start, step)
	conn, err := getMysqlConnection()
	defer releaseMysqlConnection(conn)
	if err != nil {
		errorMsg := fmt.Sprintf("%s: %s", errorMsgPrefix, err)
		lib.LogErrorln(errorMsg)
		return false, errors.New(errorMsg)
	}
	groupInfo, err := self.get(conn, group)
	if err != nil {
		errorMsg := fmt.Sprintf("%s: %s", errorMsgPrefix, err)
		lib.LogErrorln(errorMsg)
		return false, errors.New(errorMsg)
	}
	if groupInfo != nil {
		warnMsg := fmt.Sprintf("The group '%s' already exists. IGNORE group info building.", group)
		lib.LogWarnln(warnMsg)
		return false, nil
	}
	creation_dt := time.Now().Format("EEEE-MM-dd HH:mm:ss.SSS")
	rawSql := "insert `%s`(`name`, `start`, `step`, `count`, `begin`, `end`, `creation_dt`) values('%s', %v, %v, %v, %v, %v, '%v')"
	sql := fmt.Sprintf(rawSql, TABLE_NAME, group, start, step, 0, start, start, creation_dt)
	_, _, err = conn.QueryFirst(sql)
	if err != nil {
		errorMsg := fmt.Sprintf("%s (sql=%s): %s", errorMsgPrefix, sql, err)
		lib.LogErrorln(errorMsg)
		return false, errors.New(errorMsg)
	}
	return true, nil
}

func (self *mysqlStorageProvider) Get(group string) (*GroupInfo, error) {
	if len(group) == 0 {
		errorMsg := fmt.Sprint("The group name is INVALID!")
		lib.LogErrorln(errorMsg)
		return nil, errors.New(errorMsg)
	}
	errorMsgPrefix := fmt.Sprintf("Occur error when get group info (group=%v)", group)
	conn, err := getMysqlConnection()
	defer releaseMysqlConnection(conn)
	if err != nil {
		errorMsg := fmt.Sprintf("%s: %s", errorMsgPrefix, err)
		lib.LogErrorln(errorMsg)
		return nil, errors.New(errorMsg)
	}
	return self.get(conn, group)
}

func (self *mysqlStorageProvider) get(conn *autorc.Conn, group string) (*GroupInfo, error) {
	errorMsgPrefix := fmt.Sprintf("Occur error when get group info (group=%v)", group)
	rawSql := "select `start`, `step`, `count`, `begin`, `end`, `last_modified` from `%s` where `name`='%s'"
	sql := fmt.Sprintf(rawSql, TABLE_NAME, group)
	row, _, err := conn.QueryFirst(sql)
	if err != nil {
		errorMsg := fmt.Sprintf("%s (sql=%s): %s", errorMsgPrefix, sql, err)
		lib.LogErrorln(errorMsg)
		return nil, errors.New(errorMsg)
	}
	if row == nil {
		return nil, nil
	}
	start := row.Uint64(1)
	step := uint32(row.Uint(2))
	count := row.Uint64(3)
	begin := row.Uint64(4)
	end := row.Uint64(5)
	lastModified := time.Duration(row.Time(6, time.Local).Unix())
	idRange := IdRange{Begin: begin, End: end}
	groupInfo := GroupInfo{Name: group, Start: start, Step: step, Count: count, Range: idRange, LastModified: lastModified}
	return &groupInfo, nil
}

func (self *mysqlStorageProvider) Propel(group string) (*IdRange, error) {
	if len(group) == 0 {
		errorMsg := fmt.Sprint("The group name is INVALID!")
		lib.LogErrorln(errorMsg)
		return nil, errors.New(errorMsg)
	}
	sign := getSign(group)
	sign.Set()
	defer sign.Unset()
	if len(group) == 0 {
		errorMsg := fmt.Sprint("The group name is INVALID!")
		lib.LogErrorln(errorMsg)
		return nil, errors.New(errorMsg)
	}
	errorMsgPrefix := fmt.Sprintf("Occur error when propel (group=%v)", group)
	conn, err := getMysqlConnection()
	defer releaseMysqlConnection(conn)
	if err != nil {
		errorMsg := fmt.Sprintf("%s: %s", errorMsgPrefix, err)
		lib.LogErrorln(errorMsg)
		return nil, errors.New(errorMsg)
	}
	groupInfo, err := self.get(conn, group)
	if err != nil {
		errorMsg := fmt.Sprintf("%s: %s", errorMsgPrefix, err)
		lib.LogErrorln(errorMsg)
		return nil, errors.New(errorMsg)
	}
	if groupInfo == nil {
		warnMsg := fmt.Sprintf("The group '%s' not exist. IGNORE propeling.", group)
		lib.LogWarnln(warnMsg)
		return nil, nil
	}
	idRange := groupInfo.Range
	newBegin := idRange.Begin + idRange.End
	newEnd := idRange.End + uint64(groupInfo.Step)
	newCount := groupInfo.Count + uint64(1)
	rawSql := "update `%s` set `begin`=%v, `end`=%v, `count`=%v where `name`='%s'"
	sql := fmt.Sprintf(rawSql, TABLE_NAME, newBegin, newEnd, newCount, group)
	// rawSql := "update `%s` set `begin`=`end`, `end`=`end`+`step`, `count`=`count`+1 where `name`='%s'"
	// sql := fmt.Sprintf(rawSql, TABLE_NAME, group)
	_, _, err = conn.QueryFirst(sql)
	if err != nil {
		errorMsg := fmt.Sprintf("%s (sql=%s): %s", errorMsgPrefix, sql, err)
		lib.LogErrorln(errorMsg)
		return nil, errors.New(errorMsg)
	}
	newIdRange := IdRange{Begin: newBegin, End: newEnd}
	return &newIdRange, nil
}

func getSign(group string) *lib.Sign {
	if len(group) == 0 {
		return nil
	}
	sign := signMap[group]
	if sign == nil {
		sign = lib.NewSign()
		signMap[group] = sign
	}
	return sign
}
