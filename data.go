package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	srvConfig "github.com/CHESSComputing/golib/config"
	mongo "github.com/CHESSComputing/golib/mongo"
	utils "github.com/CHESSComputing/golib/utils"
	"github.com/google/uuid"
)

// helper function to validate input data record against schema
func validateData(sname string, rec map[string]any) error {
	if smgr, ok := _smgr.Map[sname]; ok {
		schema := smgr.Schema
		err := schema.Validate(rec)
		if err != nil {
			return err
		}
	} else {
		msg := fmt.Sprintf("No schema '%s' found for your record", sname)
		log.Printf("ERROR: %s, schema map %+v", msg, _smgr.Map)
		return errors.New(msg)
	}
	return nil
}

// helper function to preprocess given record
/*
func preprocess(rec map[string]any) map[string]any {
	r := make(map[string]any)
	for k, v := range rec {
		switch val := v.(type) {
		case string:
			r[strings.ToLower(k)] = strings.ToLower(val)
		case []string:
			var vals []string
			for _, vvv := range val {
				vals = append(vals, strings.ToLower(vvv))
			}
			r[strings.ToLower(k)] = vals
		case []interface{}:
			var vals []string
			for _, vvv := range val {
				s := fmt.Sprintf("%v", vvv)
				vals = append(vals, strings.ToLower(s))
			}
			r[strings.ToLower(k)] = vals
		default:
			r[strings.ToLower(k)] = val
		}
	}
	return r
}
*/

// helper function to insert data into backend DB
func insertData(sname string, rec map[string]any) error {
	// load our schema
	if _, err := _smgr.Load(sname); err != nil {
		msg := fmt.Sprintf("unable to load %s error %v", sname, err)
		log.Println("ERROR: ", msg)
		return errors.New(msg)
	}

	// check if data satisfies to one of the schema
	if err := validateData(sname, rec); err != nil {
		return err
	}
	if _, ok := rec["Date"]; !ok {
		rec["Date"] = time.Now().Unix()
	}
	rec["SchemaFile"] = sname
	rec["Schema"] = schemaName(sname)
	// main attributes to work with
	var path, cycle, beamline, btr, sample string
	if v, ok := rec["DataLocationRaw"]; ok {
		path = v.(string)
	} else {
		path = filepath.Join("/tmp", os.Getenv("USER")) // for testing purposes
		if _, err := os.Stat(path); os.IsNotExist(err) {
			log.Printf("Directory %s does not exist, will use /tmp", path)
			path = "/tmp"
		}
	}
	// log record just in case we need to debug it
	log.Printf("cycle=%v beamline=%v btr=%v sample=%v", rec["Cycle"], rec["Beamline"], rec["BTR"], rec["SampleName"])
	if v, ok := rec["Cycle"]; ok {
		cycle = v.(string)
	} else {
		cycle = fmt.Sprintf("Cycle-%s", utils.RandomString())
	}
	if v, ok := rec["Beamline"]; ok {
		switch b := v.(type) {
		case string:
			beamline = b
		case []string:
			beamline = strings.Join(b, "-")
		case []any:
			var values []string
			for _, v := range b {
				values = append(values, fmt.Sprintf("%v", v))
			}
			beamline = strings.Join(values, "-")
		}
	} else {
		beamline = fmt.Sprintf("beamline-%s", utils.RandomString())
	}
	if v, ok := rec["BTR"]; ok {
		btr = v.(string)
	} else {
		btr = fmt.Sprintf("btr-%s", utils.RandomString())
	}
	if v, ok := rec["SampleName"]; ok {
		sample = v.(string)
	} else {
		sample = fmt.Sprintf("sample-%s", utils.RandomString())
	}
	// dataset is a /cycle/beamline/BTR/sample
	dataset := fmt.Sprintf("/%s/%s/%s/%s", cycle, beamline, btr, sample)
	rec["dataset"] = dataset
	//     rec = preprocess(rec)
	// check if given path exist on file system
	_, err := os.Stat(path)
	if err == nil {
		//         log.Printf("input data, record\n%v\npath %v\n", rec, path)
		rec["path"] = path
		// generate unique id
		if _, ok := rec["did"]; !ok {
			if uuid, err := uuid.NewRandom(); err == nil {
				rec["did"] = hex.EncodeToString(uuid[:])
			} else {
				rec["did"] = fmt.Sprintf("%v", time.Now().UnixMilli())
			}
		}
		// add record to mongo DB
		var records []map[string]any
		records = append(records, rec)
		err = mongo.Upsert(
			srvConfig.Config.CHESSMetaData.MongoDB.DBName,
			srvConfig.Config.CHESSMetaData.MongoDB.DBColl,
			"dataset", records)
		if err != nil {
			log.Printf("ERROR: unable to MongoUpsert for dataset=%s path=%s, error=%v", dataset, path, err)
		}
		return err
	}
	msg := fmt.Sprintf("No files found associated with DataLocationRaw=%s", path)
	log.Printf("ERROR: %s", msg)
	return errors.New(msg)
}
