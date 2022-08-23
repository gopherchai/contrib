package dao

import (
	"bytes"
	"crypto/md5"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gopherchai/contrib/lib/db/orm"

	"github.com/Shopify/sarama"
	"github.com/go-redis/redis"
	_ "github.com/go-sql-driver/mysql"
	pkgerr "github.com/pkg/errors"

	localErr "github.com/gopherchai/contrib/lib/errors"
	base "github.com/gopherchai/contrib/lib/model"
)

const (
	TableFieldIsDeleted        = "is_deleted"
	TableFieldUpdateTime       = "updated_at"
	TableFieldCreateTime       = "created_at"
	TableFieldCreatorUserId    = "creator_user_id"
	TableFieldMaintainerUserId = "maintainer_user_id"
	TableFieldId               = "id"
	StructFieldID              = "Id"
)

func Init(dsn string, models ...interface{}) {
	err := orm.RegisterDriver("mysql", orm.DRMySQL)
	if err != nil {
		panic(err)
	}
	orm.RegisterModel(models...)
	err = orm.RegisterDataBase("default", "mysql", dsn)
	if err != nil {
		panic(err)
	}

}

var (
	defaultDataLayer     *DataLayer
	DataLayerInit        sync.Once
	defaultDataLayerInit sync.Once
	globalModInfo        = new(sync.Map)
)

type DataLayer struct {
	redisKeyPrefix string
	dbAlias        string
	rdCli          *redis.Client
	globalOrmer    orm.Ormer
	//ch             *mq.ConsumerHandler
	//producer       sarama.SyncProducer
	//genOrmer       func() (orm.Ormer, error)
	globalModInfo *sync.Map
}

func NewDataLayer(dsn, alias, prefix string, maxIdleConns, maxOpenConns int, rdCli *redis.Client,
	models ...interface{}) (*DataLayer, error) {
	DataLayerInit.Do(func() {
		err := orm.RegisterDriver("mysql", orm.DRMySQL)
		if err != nil {
			panic(err)
		}

		err = orm.RegisterDataBase("default", "mysql", dsn, maxIdleConns, maxOpenConns)
		if err != nil {
			panic(err)
		}

		orm.RegisterModel(models...,
		// new(model.Application),
		// new(model.ConfigInstance),
		// new(model.ConfigOperation),
		// new(model.Permission)
		)
	})

	o := orm.NewOrm()
	err := o.Using(alias)
	if err != nil {
		return nil, err
	}
	dl := DataLayer{
		redisKeyPrefix: prefix,
		dbAlias:        alias,
		rdCli:          rdCli,

		globalModInfo: globalModInfo,
		globalOrmer:   o,
		//producer:      producer,
	}
	baseModels := make([]base.BaseModel, 0, len(models))
	for _, m := range models {
		baseModels = append(baseModels, m.(base.BaseModel))
	}
	dl.registerTable(baseModels...)

	return &dl, nil
}

func InitDefaultDataLayer(dsn, alias, prefix string, maxIdleConns, maxOpenConns int, rdCli *redis.Client,
	producer sarama.SyncProducer) {
	var err error
	defaultDataLayerInit.Do(func() {
		globalModInfo = new(sync.Map)
		defaultDataLayer, err = NewDataLayer(dsn, alias, prefix, maxIdleConns, maxOpenConns, rdCli, producer)
		if err != nil {
			panic(err)
		}

	})
}

func GetDefaultDataLayer() *DataLayer {
	return defaultDataLayer
}

func (d *DataLayer) registerTable(mods ...base.BaseModel) {
	for _, mod := range mods {
		cols, _ := GetFieldNamesAndStructName(mod)
		d.globalModInfo.Store(mod.TableName(), cols)
	}
}

func (d *DataLayer) GetFieldNamesByTableName(name string) []string {
	val, _ := d.globalModInfo.Load(name)
	return val.([]string)
}

func (d *DataLayer) GetDB() (*sql.DB, error) {
	db, err := orm.GetDB(d.dbAlias)
	if err != nil {
		return nil, pkgerr.Wrapf(localErr.ErrSystem, "GetDb with args:%+v meet error:%+v", d.dbAlias, err)
	}
	return db, nil
}

// func (d *DataLayer) GetRedis() *redis.Client {
// 	return d.rdCli
// }

// func (d *DataLayer) LockHttpOperationInUser(operation string, userId int64) (bool, error) {
// 	key := fmt.Sprintf("%s_%s_%d", d.redisKeyPrefix, operation, userId)
// 	return d.rdCli.SetNX(key, "used", time.Minute).Result()
// }

// func (d *DataLayer) UnLockHttpOperationInUser(operation string, userId int64) error {
// 	key := fmt.Sprintf("%s_%s_%d", d.redisKeyPrefix, operation, userId)
// 	return d.rdCli.Del(key).Err()
// }

// func (d *DataLayer) GetLock(nameSpace, operation string) error {
// 	ok, err := d.rdCli.SetNX(d.redisKeyPrefix+"_"+nameSpace+"_"+operation, "locked", time.Minute).Result()
// 	if err != nil {
// 		return pkgerr.Wrapf(localErr.ErrSystem, " set lock meet error:%+v with args:%+v", err,
// 			[]interface{}{nameSpace, operation})
// 	}
// 	if !ok {
// 		return localErr.ErrOperationDoing
// 	}
// 	return nil
// }

// func (d *DataLayer) ReleaseLock(nameSpace, operation string) error {
// 	_, err := d.rdCli.Del(d.redisKeyPrefix + "_" + nameSpace + "_" + operation).Result()
// 	if err != nil {
// 		return pkgerr.Wrapf(localErr.ErrSystem, " set lock meet error:%+v with args:%+v", err,
// 			[]interface{}{nameSpace, operation})
// 	}
// 	return nil
// }

func (d *DataLayer) Create(u base.BaseModel) (int64, error) {
	id, err := d.globalOrmer.Insert(u)
	if err != nil {
		//判断是否是由于违反约束引起的错误
		return 0, pkgerr.Wrapf(err, "Create meet error:%+v with args:%+v", err, u)
	}
	return id, nil
}

//CreateModels models参数必须是*[]*Type类型 *Type实现base.BaseModel类型
func (d *DataLayer) CreateModels(models interface{}, batchSize int) (int, error) {
	num, err := d.globalOrmer.InsertMulti(batchSize, models)
	if err != nil {
		return int(num), pkgerr.Wrapf(localErr.ErrSystem, "CreateModels meet error:%+v with args:%#+v", err, models)
	}

	return int(num), nil
}

//GetModByID 要求mod必须是结构体指针，且有个字段为Id int类型
func (d *DataLayer) GetModByIDFromDB(id int64, mod base.BaseModel) error {
	mod.SetID(id)
	err := d.globalOrmer.Read(mod)
	if err != nil {
		if err == orm.ErrNoRows {
			return pkgerr.Wrapf(localErr.ErrIDNotExistInDataBase, "id:%d not exist in table:%s", id, mod.TableName())
		}
		return pkgerr.Wrapf(localErr.ErrSystem, "error:%+v with args:%v", err, id)
	}
	return nil
}

func (d *DataLayer) GetModByIDFromCacheOrDB(id int64, mod base.BaseModel) error {
	key := getModCacheKeyWithID(d.redisKeyPrefix, getDbModCachedKeySuffixWithIDAndTableName(id, mod.TableName()))
	res, err := d.rdCli.Get(key).Result()
	if err != nil {
		err = d.GetModByIDFromDB(id, mod)
		if err != nil {
			return pkgerr.Wrapf(err, "error:%+v with args:%v", err, id)
		}

		go func() {
			data, _ := json.Marshal(mod)
			_, err := d.rdCli.Set(key, string(data), time.Second*60).Result()
			if err != nil {

			}
		}()
		return nil
	}

	dec := json.NewDecoder(bytes.NewBuffer([]byte(res)))
	dec.UseNumber()
	err = dec.Decode(mod)
	if err != nil {
		return pkgerr.Wrapf(localErr.ErrSystem, "error:%+v with args:%v", err, id)
	}
	return nil
}

//GetModsWithFilterFromDB filter的key可以是驼峰的 也可是下划线的
func (d *DataLayer) GetModsWithFilterFromDB(container interface{}, tableName string, filter map[string]interface{}, pageSize, pageNo uint) error {
	limit := pageSize
	offset := pageSize * (pageNo - 1)

	if d.globalOrmer == nil {
		panic("globalOrmer nil ")
	}

	qs := d.getMatchedFilterQuerySetByTableName(tableName, filter, d.globalOrmer)
	_, err := qs.Limit(limit, offset).All(container)
	if err != nil {
		return pkgerr.Wrapf(err, "GetInfos with args:%+v,%d,%d meet error:%s", filter, pageSize, pageNo, err.Error())
	}

	return nil
}

func (d *DataLayer) GetUndeletedModsWithFilterFromDB(container interface{}, tableName string, filter map[string]interface{}, pageSize, pageNo uint) error {
	filter[TableFieldIsDeleted] = false
	return d.GetModsWithFilterFromDB(container, tableName, filter, pageSize, pageNo)
}

func (d *DataLayer) GetUndeletedModsWithoutFilterFromDB(container interface{}, tableName string, pageSize, pageNo uint) error {
	filter := make(map[string]interface{})
	return d.GetUndeletedModsWithFilterFromDB(container, tableName, filter, pageSize, pageNo)
}

func (d *DataLayer) GetModsWithoutFilterFromDB(container interface{}, tableName string, pageSize, pageNo uint) error {
	limit := pageSize
	offset := pageSize * (pageNo - 1)
	qs := d.globalOrmer.QueryTable(tableName).Limit(limit, offset)
	_, err := qs.All(container)
	if err != nil {
		return pkgerr.Wrapf(err, "GetModsWithoutFilterFromDB with args:%#+v", []interface{}{container, pageSize, pageNo})
	}
	return nil
}

//UpdateModByPKAndDeleteCache 不建议更新is_delete 被设置为true的字段
func (d *DataLayer) UpdateUndeletedModByIDAndDeleteCache(tableName string, id int64, values map[string]interface{}, mainterUserId int) (int, error) {
	delete(values, TableFieldCreateTime)
	delete(values, TableFieldCreatorUserId)
	values[TableFieldMaintainerUserId] = mainterUserId
	values = d.getMatchedFilterByTableName(tableName, values)
	num, err := d.globalOrmer.QueryTable(tableName).Filter(TableFieldId, id).Filter(TableFieldIsDeleted, false).Update(orm.Params(values))
	if err != nil {
		return 0, pkgerr.Wrapf(localErr.ErrSystem, "error:%s with args:%+v", err, []interface{}{tableName, id, values})
	}

	go d.DeleteModCacheByID(id, tableName)
	return int(num), nil
}

//UpdateModsWithFilter TODO avoid to update updateTime,createTime in values ;avoid update deleted record
func (d *DataLayer) UpdateUndeletedModsWithFilter(filter map[string]interface{}, values map[string]interface{}, mainterUserId int64, tableName string) (int, error) {

	if isdeleted, ok := filter[TableFieldIsDeleted]; ok {
		if isdeleted.(bool) {
			return 0, nil
		}
	}
	filter[TableFieldIsDeleted] = false
	qs := d.getMatchedFilterQuerySetByTableName(tableName, filter, d.globalOrmer)
	if _, ok := values[TableFieldUpdateTime]; !ok {
		values[TableFieldUpdateTime] = time.Now()
	}
	if _, ok := values[TableFieldCreateTime]; ok {
		delete(values, TableFieldCreateTime)
	}
	delete(values, TableFieldCreatorUserId)
	values = d.getMatchedFilterByTableName(tableName, values)
	values[TableFieldMaintainerUserId] = mainterUserId
	num, err := qs.Update(orm.Params(values))
	if err != nil {
		return 0, pkgerr.Wrapf(localErr.ErrSystem, "UpdateUndeletedModsWithFilter with args:%+v,%+v met error:%+v", filter, values, err)
	}

	return int(num), nil
}

func (d *DataLayer) GetUndeletedModByUniqueKeyFromDB(mod base.BaseModel, keyName string, keyValue interface{}) error {

	err := d.globalOrmer.QueryTable(mod.TableName()).Filter(keyName, keyValue).
		Filter(TableFieldIsDeleted, false).One(mod)
	if err != nil {
		if err == orm.ErrNoRows || err == orm.ErrMultiRows {
			return pkgerr.Wrapf(localErr.ErrParameter, "error:%+v with args:%+v", err, []interface{}{keyName, keyValue})
		}
		return pkgerr.Wrapf(localErr.ErrSystem, "error:%+v  with args:%+v", err, []interface{}{keyName, keyValue})
	}
	return nil
}

func (d *DataLayer) GetUndeletedModByUniqueKeyFromCacheOrDB(mod base.BaseModel, keyName string, keyValue interface{}) error {

	key := getModCacheKeyWithID(d.redisKeyPrefix, getDbModCachedKeyWithUniqueKey(keyName, keyValue, mod))
	res, err := d.rdCli.Get(key).Result()
	if err != nil {
		err := d.GetUndeletedModByUniqueKeyFromDB(mod, keyName, keyValue)
		if err != nil {
			return err
		}

		data, _ := json.Marshal(mod)
		go d.rdCli.Set(key, string(data), time.Second*60)
		return nil
	}
	err = json.Unmarshal([]byte(res), mod)
	if err != nil {
		return pkgerr.Wrapf(localErr.ErrSystem, "GetModByUniqueKeyFromCacheOrDB meet err:%+v with args:%+v", err, []interface{}{keyName, keyValue})
	}
	return nil
}

func (d *DataLayer) DeleteModCacheByID(id int64, tableName string) error {

	key := getModCacheKeyWithID(d.redisKeyPrefix, getDbModCachedKeySuffixWithIDAndTableName(id, tableName))
	return d.rdCli.Del(key).Err()
}

func (d *DataLayer) DeleteModByUniqueKey(tableName, keyName string, keyValue interface{}) (int, error) {

	num, err := d.globalOrmer.QueryTable(tableName).Filter(keyName, keyValue).Delete()
	if err != nil {
		return 0, pkgerr.Wrapf(localErr.ErrSystem, "error:%+v  with args:%+v", err, []interface{}{tableName, keyName, keyValue})
	}
	return int(num), nil
}

func (d *DataLayer) DeleteSoftModByUniqueKey(keyName string, keyValue interface{}, mainterUserId int64, tableName string) (int, error) {
	m := map[string]interface{}{
		keyName: keyValue,
	}
	value := d.getMatchedFilterByTableName(tableName, m)
	if len(value) == 0 {
		return 0, pkgerr.Wrapf(localErr.ErrParameter, "DeleteSoftModByUniqueKey with args invalid:args:%+v", []interface{}{keyName, keyValue, tableName})
	}

	num, err := d.globalOrmer.QueryTable(tableName).Filter(keyName, keyValue).Update(orm.Params{
		TableFieldIsDeleted:        true,
		TableFieldMaintainerUserId: mainterUserId,
		TableFieldUpdateTime:       time.Now(),
	})

	if err != nil {
		return 0, pkgerr.Wrapf(localErr.ErrSystem, "SoftDeleteInfoByUniqueKey meet error:%+v with args:%+v", err, []interface{}{keyName, keyValue})
	}
	return int(num), nil
}

func (d *DataLayer) DeleteSoftModsByFilter(filter map[string]interface{}, mainterUserId int64, tableName string) (int, error) {
	qs := d.getMatchedFilterQuerySetByTableName(tableName, filter, d.globalOrmer)
	num, err := qs.Update(orm.Params{
		TableFieldIsDeleted:        true,
		TableFieldMaintainerUserId: mainterUserId,
		TableFieldUpdateTime:       time.Now(),
	})
	if err != nil {
		return 0, pkgerr.Wrapf(localErr.ErrSystem, " DeleteSoftModsByFilter meet error:%+v with args:%+v", err, []interface{}{filter})
	}
	return int(num), nil
}

func (d *DataLayer) DeleteModsByFilter(filter map[string]interface{}, tableName string) (int, error) {
	qs := d.getMatchedFilterQuerySetByTableName(tableName, filter, d.globalOrmer)

	num, err := qs.Delete()
	if err != nil {
		return 0, pkgerr.Wrapf(localErr.ErrSystem, "DeleteModsByFilter meet error:%+v with args:%+v", err, filter)
	}
	return int(num), nil
}

//DeleteModWithID 必须已经设置Id字段
func (d *DataLayer) DeleteModWithID(id int64, mod base.BaseModel) (int, error) {
	mod.SetID(id)
	num, err := d.globalOrmer.Delete(mod)
	if err != nil {
		return 0, pkgerr.Wrapf(localErr.ErrSystem, "DeleteModWithID meet error:%+v with args:%+v", err, mod)
	}
	go d.DeleteModCacheByID(id, mod.TableName())

	return int(num), nil
}

func (d *DataLayer) DeleteSoftModWithID(id int64, tableName string, mainterUserId int64) (int, error) {
	num, err := d.globalOrmer.QueryTable(tableName).Filter(TableFieldId, id).Update(orm.Params{
		TableFieldIsDeleted:        true,
		TableFieldMaintainerUserId: mainterUserId,
		TableFieldUpdateTime:       time.Now(),
	})
	if err != nil {
		return 0, pkgerr.Wrapf(localErr.ErrSystem, " DeleteSoftModWithID meet error:%+v with args:%+v", err, []interface{}{id})
	}
	go d.DeleteModCacheByID(id, tableName)
	return int(num), nil
}

func (d *DataLayer) GetNumberOfModsMatchWithFilter(o orm.Ormer, tableName string, filters []map[string]interface{}) (int, error) {
	oo := d.globalOrmer
	if o != nil {
		oo = o
	}

	qs := oo.QueryTable(tableName)
	for _, filter := range filters {
		for k, v := range filter {
			qs = qs.Filter(k, v)
		}
	}
	num, err := qs.Count()
	if err != nil {
		return 0, pkgerr.Wrapf(localErr.ErrSystem, "query with args:%+v meet error:%+v", []interface{}{tableName, filters}, err)
	}
	return int(num), nil

}

func (dl *DataLayer) GetOneModWithFilterAndOrderFromCacheOrDB(o orm.Ormer, mod interface{}, tableName string, filters []map[string]interface{}, orders []string, duration time.Duration) error {
	m := map[string]interface{}{
		"filters": filters,
		"orders":  orders,
	}

	key, err := dl.getCacheKeyWithFilter([]interface{}{m}, tableName)
	if err != nil {
		return err
	}
	data, err := dl.rdCli.Get(key).Bytes()

	if err != nil || json.Unmarshal(data, mod) != nil {
		err = dl.GetOneModWithFilterAndOrder(o, mod, tableName, filters, orders)
		if err == nil {
			go func() {
				data, _ := json.Marshal(mod)
				err := dl.rdCli.Set(key, string(data), duration).Err()
				if err != nil {

				}
			}()
			return nil
		}
		return err
	}
	return nil
}

func (dl *DataLayer) GetModWithIDFromCache(container interface{}, tableName string, id int64) (err error) {
	suffix := getDbModCachedKeySuffixWithIDAndTableName(id, tableName)
	key := dl.redisKeyPrefix + "_" + suffix
	data, err := dl.rdCli.Get(key).Bytes()
	if err != nil {
		return pkgerr.Wrapf(localErr.ErrSystem, "get redis key:%+v meet error:%+v", key, err)
	}
	err = json.Unmarshal(data, container)
	if err != nil {
		return pkgerr.Wrapf(localErr.ErrSystem, "unmarshal meet error:%+v with data:%+v,table:%+s", err, string(data), tableName)
	}
	return nil
}

func (dl *DataLayer) CacheModWithIdAndTableName(container interface{}, tableName string, id int64, duration time.Duration) (err error) {
	data, err := json.Marshal(container)
	if err != nil {
		return pkgerr.Wrapf(localErr.ErrSystem, "marshal mod:%+v table:%s meet error:%+v", container, tableName, err)
	}
	suffix := getDbModCachedKeySuffixWithIDAndTableName(id, tableName)
	key := dl.redisKeyPrefix + "_" + suffix
	err = dl.rdCli.Set(key, string(data), duration).Err()
	if err != nil {
		return pkgerr.Wrapf(localErr.ErrSystem, "set redis key:%s meet error:%+v", key, err)
	}

	return nil
}

func (dl *DataLayer) GetNumberOfModsMatchWithFilterFromCacheOrDB(o orm.Ormer, tableName string, filters []map[string]interface{}, duration time.Duration) (int, error) {
	key, err := dl.getCacheKeyWithFilter([]interface{}{filters}, tableName)
	if err != nil {
		return 0, err
	}

	num, err := dl.rdCli.Get(key).Int()
	if err != nil {
		num, err := dl.GetNumberOfModsMatchWithFilter(o, tableName, filters)
		if err != nil {
			return 0, err
		}
		go func() {
			err := dl.rdCli.Set(key, num, duration).Err()
			if err != nil {
				//TODO 打印错误
			}
		}()
		return num, nil
	}
	return num, nil
}

func (d *DataLayer) GetTotalUndeletedModNumByFilter(filter map[string]interface{}, tableName string) (int, error) {
	filter[TableFieldIsDeleted] = false
	qs := d.getMatchedFilterQuerySetByTableName(tableName, filter, d.globalOrmer)
	cnt, err := qs.Count()
	if err != nil {
		return 0, pkgerr.Wrapf(localErr.ErrSystem, "GetTotalModNumByFilter with args:%+v", []interface{}{filter, tableName})
	}
	return int(cnt), nil
}

func (dl *DataLayer) GetModsFromDb(o orm.Ormer, container interface{}, tableName string, filter []map[string]interface{}, orders []string, pageNo, pageSize uint, duration time.Duration, columns []string) error {
	qs := o.QueryTable(tableName)

	for _, m := range filter {
		for k, v := range m {
			qs = qs.Filter(k, v)
		}
	}

	if len(orders) > 0 {
		qs = qs.OrderBy(orders...)
	}
	if pageNo == 0 {
		pageNo = 1
	}
	_, err := qs.Limit(pageSize).Offset((pageNo-1)*pageSize).All(container, columns...)
	if err != nil {
		return pkgerr.Wrapf(localErr.ErrSystem, "query "+tableName+" meet error:%+v with args:%+v", err, []interface{}{filter, orders, pageNo, pageSize, columns})
	}
	data, _ := json.Marshal(container)
	go dl.cacheData([]interface{}{filter, orders, pageNo, pageSize, columns}, tableName, string(data), duration)
	return nil
}

func (dl *DataLayer) cacheData(args []interface{}, tableName, value string, duration time.Duration) error {
	key, err := dl.getCacheKeyWithFilter(args, tableName)
	if err != nil {
		return err
	}
	err = dl.rdCli.Set(key, value, duration).Err()
	if err != nil {
		return pkgerr.Wrapf(localErr.ErrSystem, "cache key:%s with value:%s meet error:%+v with args:%+v", key, value, err, []interface{}{args, tableName})
	}
	return nil
}

func (dl *DataLayer) getCacheKeyWithFilter(args []interface{}, tableName string) (string, error) {
	data, err := json.Marshal(args)
	if err != nil {
		return "", pkgerr.Wrapf(localErr.ErrSystem, "json marsharl meet error:%+v with  args:%+v", err, args)
	}
	h := md5.New()
	_, err = h.Write(data)
	if err != nil {
		return "", pkgerr.Wrapf(localErr.ErrSystem, "campute md5 meet error:%+v with  args:%+v", err, args)
	}
	sum := h.Sum(nil)
	return dl.redisKeyPrefix + "_" + tableName + "_" + fmt.Sprintf("%x", sum), nil
}

func (dl *DataLayer) GetModsFromCache(o orm.Ormer, container interface{}, tableName string, filter []map[string]interface{}, orders []string, pageNo, pageSize uint, columns []string) error {

	args := []interface{}{filter, orders, pageNo, pageSize, columns}
	key, err := dl.getCacheKeyWithFilter(args, tableName)
	if err != nil {
		return err
	}
	val, err := dl.rdCli.Get(key).Result()
	if err != nil {
		return pkgerr.Wrapf(localErr.ErrSystem, "get key:%s from redis meet error:%+v with args:%+v", key, err, []interface{}{args, tableName})
	}
	err = json.Unmarshal([]byte(val), container)
	if err != nil {
		return pkgerr.Wrapf(localErr.ErrSystem, "unmarshal val:%+v to container meet error:%+v", val, err)
	}
	return nil
}

func (dl *DataLayer) GetModsFromCacheOrDB(o orm.Ormer, container interface{}, tableName string, filter []map[string]interface{}, orders []string, pageNo, pageSize uint, duration time.Duration, columns []string) error {
	err := dl.GetModsFromCache(o, container, tableName, filter, orders, pageNo, pageSize, columns)
	if err != nil {
		err := dl.GetModsFromDb(o, container, tableName, filter, orders, pageNo, pageSize, duration, columns)
		if err != nil {
			return err
		}
		go func() {
			data, err := json.Marshal(container)
			if err != nil {

				//TODO 打印错误tableName, []interface{}{filter, orders, pageNo, pageSize, duration, columns})
				return
			}
			err = dl.cacheData([]interface{}{filter, orders, pageNo, pageSize, columns}, tableName, string(data), duration)
			if err != nil {

			}
		}()

	}
	return nil
}

func (d *DataLayer) getMatchedFilterQuerySetByTableName(tableName string, filter map[string]interface{}, o orm.Ormer) orm.QuerySeter {
	m := d.getMatchedFilterByTableName(tableName, filter)
	qs := o.QueryTable(tableName)
	res := d.GetFieldNamesByTableName(tableName)
	for _, k := range res {
		if v, ok := m[snakeString(k)]; ok {
			qs = qs.Filter(snakeString(k), v)
		}
	}
	return qs
}

func (d *DataLayer) GetMatchedFilterByTableName(tableName string, filter map[string]interface{}) map[string]interface{} {
	return d.getMatchedFilterByTableName(tableName, filter)
}

func (d *DataLayer) getMatchedFilterByTableName(tableName string, filter map[string]interface{}) map[string]interface{} {

	snakeFilter := make(map[string]interface{})
	for k, v := range filter {
		snakeFilter[snakeString(k)] = v
	}
	m := make(map[string]interface{})
	res := d.GetFieldNamesByTableName(tableName)

	for _, s := range res {
		if val, ok := snakeFilter[s]; ok {
			m[snakeString(s)] = val
		}
		if val, ok := snakeFilter[snakeString(s)]; ok {
			m[snakeString(s)] = val
		}
	}

	return m
}

func (d *DataLayer) GetModsWithFilterAndOrder(o orm.Ormer, mods interface{}, tableName string, filters []map[string]interface{}, orderFields []string) error {
	return d.GetModsWithFilterAndOrderByPage(o, mods, tableName, filters, orderFields, 1, 1000)
}

func (d *DataLayer) GetModsWithFilterAndOrderByPage(o orm.Ormer, mods interface{}, tableName string, filters []map[string]interface{}, orderFields []string, pageNo, pageSize uint) error {
	oo := d.globalOrmer
	if o != nil {
		oo = o
	}
	qs := oo.QueryTable(tableName)
	for _, filter := range filters {
		for k, v := range filter {
			qs = qs.Filter(k, v)
		}
	}

	if len(orderFields) > 0 {
		qs = qs.OrderBy(orderFields...)
	}
	if pageNo == 0 {
		pageNo = 1
	}
	_, err := qs.Limit(pageSize).Offset((pageNo - 1) * pageSize).All(mods)
	if err != nil {
		return pkgerr.Wrapf(localErr.ErrSystem, "query table :%s with args %+v meet error:%+v", tableName, []interface{}{
			filters, orderFields, pageNo, pageSize,
		}, err)
	}
	return nil

}

func (d *DataLayer) GetOneModWithFilterAndOrder(o orm.Ormer, mod interface{}, tableName string, filters []map[string]interface{}, ordersFields []string) error {
	oo := d.globalOrmer
	if o != nil {
		oo = o
	}
	qs := oo.QueryTable(tableName)
	for _, filter := range filters {
		for k, v := range filter {
			qs = qs.Filter(k, v)
		}
	}

	if len(ordersFields) > 0 {
		qs = qs.OrderBy(ordersFields...)
	}
	err := qs.One(mod)
	if err != nil {
		if err == orm.ErrNoRows {

			return localErr.ErrQualifiedRecordNotFound
		}
		return pkgerr.Wrapf(localErr.ErrSystem, "query table:%+v with args:%+v meet error:%+v", tableName, []interface{}{
			filters, ordersFields,
		}, err)
	}
	return nil

}

func (d *DataLayer) DeleteMatchedMods(o orm.Ormer, tableName string, filters []map[string]interface{}) (int, error) {
	oo := d.globalOrmer
	if o != nil {
		oo = o
	}
	qs := oo.QueryTable(tableName)
	for _, filter := range filters {
		for k, v := range filter {
			qs = qs.Filter(k, v)
		}
	}
	num, err := qs.Delete()
	if err != nil {
		return 0, pkgerr.Wrapf(localErr.ErrSystem, "delete from :%s meet error:%+v with args:%+v", tableName, err, filters)
	}
	return int(num), nil
}

func (d *DataLayer) GetOrmer() orm.Ormer {
	return d.globalOrmer
}

func (d *DataLayer) GenOrmer() orm.Ormer {
	return orm.NewOrm()
}

func (d *DataLayer) Insert(o orm.Ormer, mod interface{}) (id int64, err error) {
	oo := d.globalOrmer
	if o != nil {
		oo = o
	}
	id, err = oo.Insert(mod)
	if err != nil {
		data, _ := json.Marshal(mod)
		return 0, pkgerr.Wrapf(localErr.ErrSystem, "insert %s to db meet error:%+v", string(data), err)
	}
	return
}

func getModCacheKeyWithID(service, idTableKey string) string {
	return service + "_" + idTableKey
}

func getDbModCachedKeySuffixWithIDAndTableName(id int64, tableName string) string {
	return fmt.Sprintf("db_%s_id_%d", tableName, id)
}

//GetFieldNames 从container中提取结构体的所有字段名称 仅支持[]*structName,*structName和*[]structName类型
func GetFieldNamesAndStructName(container interface{}) ([]string, string) {
	res := make([]string, 0, 0)
	var tableName string
	val := reflect.ValueOf(container)
	ind := reflect.Indirect(val)

	var num int
	switch val.Kind() {
	case reflect.Ptr:

		if ind.Kind() == reflect.Slice {
			typ := ind.Type().Elem()
			switch typ.Kind() {
			case reflect.Ptr:
				num = ind.Type().NumField()
				for i := 0; i < num; i++ {
					//判断是否是ORMCommon类型
					fieldName := ind.Type().Field(i).Name

					if fieldName == "OrmCommon" {
						res = append(res, []string{"CreatorUserId", "MaintainerUserId", "CreatedAt", "UpdatedAt", "IsDeleted"}...)
					} else {
						res = append(res, fieldName)
					}

				}
				tableName = ind.Type().Name()
			case reflect.Struct:
				num = typ.NumField()
				for i := 0; i < num; i++ {

					if typ.Field(i).Name == "OrmCommon" {
						res = append(res, []string{"CreatorUserId", "MaintainerUserId", "CreatedAt", "UpdatedAt", "IsDeleted"}...)
					} else {
						res = append(res, typ.Field(i).Name)
					}
				}
				tableName = typ.Name()
			default:
				panic("invalid container")
			}
		} else if ind.Kind() == reflect.Struct {
			num = ind.NumField()

			for i := 0; i < num; i++ {
				if reflect.Indirect(ind).Type().Field(i).Name == "OrmCommon" {
					res = append(res, []string{"CreatorUserId", "MaintainerUserId", "CreatedAt", "UpdatedAt", "IsDeleted"}...)
				} else {
					res = append(res, reflect.Indirect(ind).Type().Field(i).Name)
				}
			}
			tableName = ind.Type().Name()
		} else {
			panic("invalid container")
		}
	case reflect.Slice:

		typ := ind.Type().Elem()
		switch typ.Kind() {
		case reflect.Ptr:

			// typ := ind.Type().Elem()
			// switch typ.Kind() {
			// case reflect.Ptr:
			t := typ.Elem()
			num := t.NumField()
			for i := 0; i < num; i++ {
				res = append(res, t.Field(i).Name)
			}
			tableName = t.Name()

		case reflect.Struct:

			num = typ.NumField()
			for i := 0; i < num; i++ {
				res = append(res, typ.Field(i).Name)
			}
			tableName = typ.Name()
		// default:
		// 		panic("invalid container")
		// 	}

		default:
			panic("invalid type ")
		}
	}

	return res, tableName
}

//getMatchedField 从map中提取出于mod对应的字段的key value 对,mod为结构体指针
func getMatchedField(mod interface{}, m map[string]interface{}) map[string]interface{} {
	matched := make(map[string]interface{})
	v := reflect.ValueOf(mod)
	ind := reflect.Indirect(v)
	t := ind.Type()
	num := t.NumField()
	for i := 0; i < num; i++ {
		name := snakeString(t.Field(i).Name)
		for k, v := range m {
			if snakeString(k) == name {
				matched[name] = v
			}
		}
	}
	return matched
}

// func (d *DataLayer) getMatchedFilterByTableName(tableName string, filter map[string]interface{}) map[string]interface{} {

// 	m := make(map[string]interface{})
// 	res := d.GetFieldNamesByTableName(tableName)
// 	for _, s := range res {
// 		if val, ok := filter[s]; ok {
// 			m[snakeString(s)] = val
// 		}
// 		if val, ok := filter[snakeString(s)]; ok {
// 			m[snakeString(s)] = val
// 		}
// 	}
// 	return m
// }

// snake string, XxYy to xx_yy , XxYY to xx_y_y
func snakeString(s string) string {
	data := make([]byte, 0, len(s)*2)
	j := false
	num := len(s)
	for i := 0; i < num; i++ {
		d := s[i]
		if i > 0 && d >= 'A' && d <= 'Z' && j {
			data = append(data, '_')
		}
		if d != '_' {
			j = true
		}
		data = append(data, d)
	}
	return strings.ToLower(string(data[:]))
}

// camel string, xx_yy to XxYy
func camelString(s string) string {
	data := make([]byte, 0, len(s))
	flag, num := true, len(s)-1
	for i := 0; i <= num; i++ {
		d := s[i]
		if d == '_' {
			flag = true
			continue
		} else if flag {
			if d >= 'a' && d <= 'z' {
				d = d - 32
			}
			flag = false
		}
		data = append(data, d)
	}
	return string(data[:])
}

func getDbModCachedKeyWithUniqueKey(keyName string, keyValue interface{}, mod interface{}) string {
	t := reflect.TypeOf(mod)
	var modName string
	switch t.Kind() {
	case reflect.Ptr:
		modName = reflect.Indirect(reflect.ValueOf(mod)).Type().Name()
	case reflect.Struct:
		modName = reflect.TypeOf(mod).Name()
	default:
		panic(fmt.Sprintf("invalid mod:%+v", mod))
	}
	return fmt.Sprintf("db_%s_unique:%s_%v", snakeString(modName), keyName, keyValue)
}
