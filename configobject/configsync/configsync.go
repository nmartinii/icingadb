// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package configsync

import (
	"encoding/json"
	"fmt"
	"github.com/Icinga/icingadb/configobject"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/ha"
	"github.com/Icinga/icingadb/jsondecoder"
	"github.com/Icinga/icingadb/supervisor"
	"github.com/Icinga/icingadb/utils"
	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
	"sync"
	"sync/atomic"
	"time"
)

type Checksums struct {
	NameChecksum       string `json:"name_checksum"`
	PropertiesChecksum string `json:"checksum"`
}

// Operator is the main worker for each config type. It takes a reference to a supervisor super, holding all required
// connection information and other control mechanisms, a channel chHA, which informs the Operator of the current HA
// state, and a ObjectInformation reference defining the type and providing the necessary factories.
func Operator(super *supervisor.Supervisor, chHA chan int, objectInformation *configobject.ObjectInformation) error {
	//insert, update, delete := GetDelta(super, objectInformation)
	//log.Infof("%s - Delta: (Insert: %d, Maybe Update: %d, Delete: %d)", objectInformation.ObjectType, len(insert), len(update), len(delete))

	var (
		// If this IcingaDB-Instance looses responsibility, this channel will be
		// closed, resulting in a shutdown of all underlying workers
		done chan struct{}
		// Used by this Operator to provide the InsertPrepWorker with IDs to insert
		// Operator -> InsertPrepWorker
		chInsert chan []string
		// Used by the JsonDecodePool to provide the InsertExecWorker with decoded rows, ready to be inserted
		// JsonDecodePool -> InsertExecWorker
		chInsertBack chan []connection.Row
		// Used by this Operator to provide the DeleteExecWorker with IDs to delete
		// Operator -> DeleteExecWorker
		chDelete chan []string
		// Used by this Operator to provide the UpdateCompWorker with IDs to compare
		// Operator -> UpdateCompWorker
		chUpdateComp chan []string
		// Used by the UpdateCompWorker to provide the UpdatePrepWorker with IDs that have to be updated
		// UpdateCompWorker -> UpdatePrepWorker
		chUpdate chan []string
		// Used by the JsonDecodePool to provide the UpdateExecWorker with decoded rows, ready to be updated
		// JsonDecodePool -> UpdateExecWorker
		chUpdateBack chan []connection.Row
		wgInsert     *sync.WaitGroup
		wgDelete     *sync.WaitGroup
		wgUpdate     *sync.WaitGroup
	)
	log.Debugf("%s: Ready", objectInformation.ObjectType)
	for msg := range chHA {
		switch msg {
		// Icinga 2 probably restarted or died, stop operations and tell all workers to shut down.
		case ha.Notify_StopSync:
			if done != nil {
				log.Debugf("%s: Lost responsibility", objectInformation.ObjectType)
				close(done)
				done = nil
			}
		// Starts up the whole sync process.
		case ha.Notify_StartSync:
			if done != nil {
				continue
			}

			super.WgConfigSync.Add(3)

			log.Debugf("%s: Got responsibility", objectInformation.ObjectType)

			//TODO: This should only be done, if HA was taken over from another instance
			insert, update, delete := GetDelta(super, objectInformation)
			//log.Infof("%s - Delta: (Insert: %d, Maybe Update: %d, Delete: %d)", objectInformation.ObjectType, len(insert), len(update), len(delete))

			// Clean up all channels and wait groups for a fresh config dump
			done = make(chan struct{})
			chInsert = make(chan []string)
			chInsertBack = make(chan []connection.Row)
			chDelete = make(chan []string)
			chUpdateComp = make(chan []string)
			chUpdate = make(chan []string)
			chUpdateBack = make(chan []connection.Row)
			wgInsert = &sync.WaitGroup{}
			wgDelete = &sync.WaitGroup{}
			wgUpdate = &sync.WaitGroup{}

			updateCounter := new(uint32)

			go InsertPrepWorker(super, objectInformation, done, chInsert, chInsertBack)
			go InsertExecWorker(super, objectInformation, done, chInsertBack, wgInsert)

			go DeleteExecWorker(super, objectInformation, done, chDelete, wgDelete)

			go UpdateCompWorker(super, objectInformation, done, chUpdateComp, chUpdate, wgUpdate)
			go UpdatePrepWorker(super, objectInformation, done, chUpdate, chUpdateBack)
			go UpdateExecWorker(super, objectInformation, done, chUpdateBack, wgUpdate, updateCounter)

			go RuntimeUpdateWorker(super, objectInformation, done, chInsert, chUpdate, chDelete, wgInsert, wgUpdate, wgDelete)

			waitOrKill := func(wg *sync.WaitGroup, done chan struct{}) (kill bool) {
				waitDone := make(chan bool)
				go func() {
					wg.Wait()
					close(waitDone)
				}()

				select {
				case <-waitDone:
					return false
				case <-done:
					return true
				}
			}

			go func() {
				defer super.WgConfigSync.Done()

				benchmarc := utils.NewBenchmark()
				wgInsert.Add(len(insert))

				// Provide the InsertPrepWorker with IDs to insert
				chInsert <- insert

				// Wait for all IDs to be inserted into MySQL
				kill := waitOrKill(wgInsert, done)
				benchmarc.Stop()
				if !kill && len(insert) > 0 {
					log.WithFields(log.Fields{
						"type":      objectInformation.ObjectType,
						"count":     len(insert),
						"benchmark": benchmarc.String(),
						"action":    "insert",
					}).Infof("Inserted %v %ss in %v", len(insert), objectInformation.ObjectType, benchmarc.String())
				}
			}()

			go func() {
				defer super.WgConfigSync.Done()

				benchmarc := utils.NewBenchmark()
				wgDelete.Add(len(delete))

				// Provide the DeleteExecWorker with IDs to delete
				chDelete <- delete

				// Wait for all IDs to be deleted from MySQL
				kill := waitOrKill(wgDelete, done)
				benchmarc.Stop()
				if !kill && len(delete) > 0 {
					log.WithFields(log.Fields{
						"type":      objectInformation.ObjectType,
						"count":     len(delete),
						"benchmark": benchmarc.String(),
						"action":    "delete",
					}).Infof("Deleted %v %ss in %v", len(delete), objectInformation.ObjectType, benchmarc.String())
				}
			}()

			if objectInformation.HasChecksum {
				go func() {
					defer super.WgConfigSync.Done()

					benchmarc := utils.NewBenchmark()
					wgUpdate.Add(len(update))

					// Provide the UpdateCompWorker with IDs to compare
					chUpdateComp <- update

					// Wait for all IDs to be update in MySQL
					kill := waitOrKill(wgUpdate, done)
					benchmarc.Stop()
					if !kill && atomic.LoadUint32(updateCounter) > 0 {
						log.WithFields(log.Fields{
							"type":      objectInformation.ObjectType,
							"count":     atomic.LoadUint32(updateCounter),
							"benchmark": benchmarc.String(),
							"action":    "update",
						}).Infof("Updated %v %ss in %v", atomic.LoadUint32(updateCounter), objectInformation.ObjectType, benchmarc.String())
					}
				}()
			} else {
				super.WgConfigSync.Done()
			}
		}
	}

	return nil
}

// GetDelta takes the ObjectInformation (host, service, checkcommand, etc.) and fetches the ids from MySQL and Redis. It
// returns three string slices:
// 1. IDs which are in the Redis but not in the MySQL (to insert)
// 2. IDs which are in both (to possibly update)
// 3. IDs which are in the MySQL but not the Redis (to delete)
func GetDelta(super *supervisor.Supervisor, objectInformation *configobject.ObjectInformation) ([]string, []string, []string) {
	var (
		redisIds []string
		mysqlIds []string
		wg       = sync.WaitGroup{}
	)

	//get ids from redis
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		res, err := super.Rdbw.HKeys(fmt.Sprintf("icinga:config:%s", objectInformation.RedisKey)).Result()
		if err != nil {
			super.ChErr <- err
			return
		}
		redisIds = res
	}()

	//get ids from mysql
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		super.EnvLock.Lock()
		mysqlIds, err = super.Dbw.SqlFetchIds(super.EnvId, objectInformation.ObjectType, objectInformation.PrimaryMySqlField)
		super.EnvLock.Unlock()
		if err != nil {
			super.ChErr <- err
			return
		}
	}()

	wg.Wait()
	return utils.Delta(redisIds, mysqlIds)
}

// InsertPrepWorker fetches config for IDs(chInsert) from Redis, wraps it into JsonDecodePackages and throws it into the JsonDecodePool
func InsertPrepWorker(super *supervisor.Supervisor, objectInformation *configobject.ObjectInformation, done chan struct{}, chInsert <-chan []string, chInsertBack chan<- []connection.Row) {
	log.Debugf("%s: Insert preparation worker started", objectInformation.ObjectType)
	defer log.Debugf("%s: Insert preparation worker stopped", objectInformation.ObjectType)

	prep := func(chunk *connection.ConfigChunk) {
		pkgs := jsondecoder.JsonDecodePackages{
			ChBack: chInsertBack,
		}
		for i, key := range chunk.Keys {
			if chunk.Configs[i] == nil {
				continue
			}

			pkg := jsondecoder.JsonDecodePackage{
				Id:         key,
				ConfigRaw:  chunk.Configs[i].(string),
				Factory:    objectInformation.Factory,
				ObjectType: objectInformation.ObjectType,
			}

			if chunk.Checksums[i] != nil {
				pkg.ChecksumsRaw = chunk.Checksums[i].(string)
			}

			pkgs.Packages = append(pkgs.Packages, pkg)
		}

		super.ChDecode <- &pkgs
	}

	for keys := range chInsert {
		select {
		case _, ok := <-done:
			if !ok {
				return
			}
		default:
		}

		ch := super.Rdbw.PipeConfigChunks(done, keys, objectInformation.RedisKey)
		go func() {
			for chunk := range ch {
				go prep(chunk)
			}
		}()
	}
}

// InsertExecWorker gets decoded connection.Row objects from the JsonDecodePool and inserts them into MySQL
func InsertExecWorker(super *supervisor.Supervisor, objectInformation *configobject.ObjectInformation, done chan struct{}, chInsertBack <-chan []connection.Row, wg *sync.WaitGroup) {
	log.Debugf("%s: Insert execution worker started", objectInformation.ObjectType)
	defer log.Debugf("%s: Insert execution worker stopped", objectInformation.ObjectType)

	for rows := range chInsertBack {
		select {
		case _, ok := <-done:
			if !ok {
				return
			}
		default:
		}

		go func(rows []connection.Row) {
			super.ChErr <- super.Dbw.SqlBulkInsert(rows, objectInformation.BulkInsertStmt)
			rowLen := len(rows)
			wg.Add(-rowLen)
			ConfigSyncInsertsTotal.WithLabelValues(objectInformation.ObjectType).Add(float64(rowLen))
		}(rows)
	}
}

// DeleteExecWorker deletes IDs(chDelete) from MySQL
func DeleteExecWorker(super *supervisor.Supervisor, objectInformation *configobject.ObjectInformation, done chan struct{}, chDelete <-chan []string, wg *sync.WaitGroup) {
	log.Debugf("%s: Delete execution worker started", objectInformation.ObjectType)
	defer log.Debugf("%s: Delete execution worker stopped", objectInformation.ObjectType)

	for keys := range chDelete {
		select {
		case _, ok := <-done:
			if !ok {
				return
			}
		default:
		}

		go func(keys []string) {
			super.ChErr <- super.Dbw.SqlBulkDelete(keys, objectInformation.BulkDeleteStmt)
			rowLen := len(keys)
			wg.Add(-rowLen)
			ConfigSyncDeletesTotal.WithLabelValues(objectInformation.ObjectType).Add(float64(rowLen))
		}(keys)
	}
}

// UpdateCompWorker gets IDs(chUpdateComp) that might need an update, fetches the corresponding checksums for Redis and MySQL,
// compares them and inserts changed IDs into chUpdate.
func UpdateCompWorker(super *supervisor.Supervisor, objectInformation *configobject.ObjectInformation, done chan struct{}, chUpdateComp <-chan []string, chUpdate chan<- []string, wg *sync.WaitGroup) {
	log.Debugf("%s: Update comparison worker started", objectInformation.ObjectType)
	defer log.Debugf("%s: Update comparison worker stopped", objectInformation.ObjectType)

	prep := func(chunk *connection.ChecksumChunk, mysqlChecksums map[string]map[string]string) {
		changed := make([]string, 0)
		for i, key := range chunk.Keys {
			if chunk.Checksums[i] == nil {
				continue
			}

			//TODO: Check if this can be done better (json should not be processed in this func)
			redisChecksums := &Checksums{}
			err := json.Unmarshal([]byte(chunk.Checksums[i].(string)), redisChecksums)
			if err != nil {
				super.ChErr <- err
			}

			if redisChecksums.PropertiesChecksum != mysqlChecksums[key]["properties_checksum"] {
				changed = append(changed, key)
			} else {
				wg.Done()
			}
		}
		chUpdate <- changed
	}

	for keys := range chUpdateComp {
		select {
		case _, ok := <-done:
			if !ok {
				return
			}
		default:
		}

		ch := super.Rdbw.PipeChecksumChunks(done, keys, objectInformation.RedisKey)
		checksums, err := super.Dbw.SqlFetchChecksums(objectInformation.ObjectType, keys)
		if err != nil {
			super.ChErr <- err
		}

		go func() {
			for chunk := range ch {
				go prep(chunk, checksums)
			}
		}()
	}
}

// UpdatePrepWorker fetches config for IDs(chUpdate) from Redis, wraps it into JsonDecodePackages and throws it into the JsonDecodePool
func UpdatePrepWorker(super *supervisor.Supervisor, objectInformation *configobject.ObjectInformation, done chan struct{}, chUpdate <-chan []string, chUpdateBack chan<- []connection.Row) {
	log.Debugf("%s: Update preparation worker started", objectInformation.ObjectType)
	defer log.Debugf("%s: Update preparation worker stopped", objectInformation.ObjectType)

	prep := func(chunk *connection.ConfigChunk) {
		pkgs := jsondecoder.JsonDecodePackages{
			ChBack: chUpdateBack,
		}
		for i, key := range chunk.Keys {
			if chunk.Configs[i] == nil || chunk.Checksums[i] == nil {
				continue
			}
			pkg := jsondecoder.JsonDecodePackage{
				Id:           key,
				ChecksumsRaw: chunk.Checksums[i].(string),
				ConfigRaw:    chunk.Configs[i].(string),
				Factory:      objectInformation.Factory,
				ObjectType:   objectInformation.ObjectType,
			}
			pkgs.Packages = append(pkgs.Packages, pkg)
		}

		super.ChDecode <- &pkgs
	}

	for keys := range chUpdate {
		select {
		case _, ok := <-done:
			if !ok {
				return
			}
		default:
		}

		ch := super.Rdbw.PipeConfigChunks(done, keys, objectInformation.RedisKey)
		go func() {
			for chunk := range ch {
				go prep(chunk)
			}
		}()
	}
}

// UpdateExecWorker gets decoded connection.Row objects from the JsonDecodePool and updates them in MySQL
func UpdateExecWorker(super *supervisor.Supervisor, objectInformation *configobject.ObjectInformation, done chan struct{}, chUpdateBack <-chan []connection.Row, wg *sync.WaitGroup, updateCounter *uint32) {
	log.Debugf("%s: Update execution worker started", objectInformation.ObjectType)
	defer log.Debugf("%s: Update execution worker stopped", objectInformation.ObjectType)

	for rows := range chUpdateBack {
		select {
		case _, ok := <-done:
			if !ok {
				return
			}
		default:
		}

		go func(rows []connection.Row) {
			super.ChErr <- super.Dbw.SqlBulkUpdate(rows, objectInformation.BulkUpdateStmt)
			rowLen := len(rows)
			wg.Add(-rowLen)
			atomic.AddUint32(updateCounter, uint32(rowLen))
			ConfigSyncUpdatesTotal.WithLabelValues(objectInformation.ObjectType).Add(float64(rowLen))
		}(rows)
	}
}

func RuntimeUpdateWorker(super *supervisor.Supervisor, objectInformation *configobject.ObjectInformation, done chan struct{}, chInsert chan []string, chUpdate chan []string, chDelete chan []string, wgInsert *sync.WaitGroup, wgUpdate *sync.WaitGroup, wgDelete *sync.WaitGroup) {
	subscription := super.Rdbw.Subscribe()
	defer subscription.Close()

	deletePubSubString := "icinga:config:delete:" + objectInformation.RedisKey
	updatePubSubString := "icinga:config:update:" + objectInformation.RedisKey

	if err := subscription.Subscribe(deletePubSubString, updatePubSubString); err != nil {
		super.ChErr <- err
	}

	var currentUpdatePackage []string
	var currentDeletePackage []string
	updateMutex := sync.Mutex{}
	deleteMutex := sync.Mutex{}

	insertCurrentUpdatePackage := func() {
		updateLen := len(currentUpdatePackage)

		if objectInformation.HasChecksum {
			chUpdate <- currentUpdatePackage
			wgUpdate.Add(updateLen)
		} else {
			chInsert <- currentUpdatePackage
			wgInsert.Add(updateLen)
		}

		currentUpdatePackage = []string{}

		log.WithFields(log.Fields{
			"type":   objectInformation.ObjectType,
			"action": "runtime insert/update",
		}).Infof("Inserting %v %ss on runtime update", updateLen, objectInformation.ObjectType)
	}

	insertCurrentDeletePackage := func() {
		deleteLen := len(currentDeletePackage)
		chDelete <- currentDeletePackage
		wgDelete.Add(deleteLen)
		currentDeletePackage = []string{}

		log.WithFields(log.Fields{
			"type":   objectInformation.ObjectType,
			"action": "runtime delete",
		}).Infof("Deleting %v %ss on runtime update", deleteLen, objectInformation.ObjectType)
	}

	ticker1s := time.NewTicker(time.Second)

	msgCh := subscription.ChannelSize(250000)

	for {
		select {
		case _, ok := <-done:
			if !ok {
				return
			}
		case message := <-msgCh:
			go func(msg *redis.Message) {
				objectId := msg.Payload
				switch msg.Channel {
				case updatePubSubString:
					updateMutex.Lock()
					currentUpdatePackage = append(currentUpdatePackage, objectId)
					if len(currentUpdatePackage) >= 1000 {
						insertCurrentUpdatePackage()
					}
					updateMutex.Unlock()
				case deletePubSubString:
					deleteMutex.Lock()
					currentDeletePackage = append(currentDeletePackage, objectId)
					if len(currentDeletePackage) >= 1000 {
						insertCurrentDeletePackage()
					}
					deleteMutex.Unlock()
				}
			}(message)
		case <-ticker1s.C:
			updateMutex.Lock()
			updateLen := len(currentUpdatePackage)
			if updateLen > 0 {
				insertCurrentUpdatePackage()
			}
			updateMutex.Unlock()

			deleteMutex.Lock()
			deleteLen := len(currentDeletePackage)
			if deleteLen > 0 {
				insertCurrentDeletePackage()
			}
			deleteMutex.Unlock()
		}
	}
}
