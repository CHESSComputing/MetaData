package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	srvConfig "github.com/CHESSComputing/golib/config"
	"github.com/CHESSComputing/golib/globus"
	utils "github.com/CHESSComputing/golib/utils"
)

// HistoryRecord represents history part of metadata record
type HistoryRecord struct {
	User      string
	Timestamp int64
}

// String provives string representation of history record
func (h *HistoryRecord) String() string {
	if val, err := json.Marshal(h); err == nil {
		return string(val)
	}
	return fmt.Sprintf("%v", h)
}

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

// helper function to create globus link
func globusLink(rec map[string]any) (string, error) {
	var path string
	locAttrs := srvConfig.Config.CHESSMetaData.DataLocationAttributes
	for _, attr := range locAttrs {
		if v, ok := rec[attr]; ok {
			path = v.(string)
			break
		}
	}
	if path == "" {
		msg := fmt.Sprintf("no data location attributes %v found in meta-data record", locAttrs)
		return "", errors.New(msg)
	}
	pat := "CHESS Raw"
	gurl, err := globus.ChessGlobusLink(pat, path)
	return gurl, err
}

// helper function to update DOI part of metadata record
func updateDoiData(did, doi string, public bool) error {
	spec := make(map[string]any)
	spec["did"] = did
	spec["doi"] = doi
	record := make(map[string]any)
	record["doi_public"] = true
	err := metaDB.Update(
		srvConfig.Config.CHESSMetaData.MongoDB.DBName,
		srvConfig.Config.CHESSMetaData.MongoDB.DBColl,
		spec, record)
	return err

}

// helper function to archive record
func archiveRecord(rec map[string]any) error {
	var did string
	if val, ok := rec["did"]; ok {
		did = val.(string)
	} else {
		msg := fmt.Sprintf("unable to find metadata record for did=%s", did)
		log.Println("ERROR:", msg, "record", rec)
		return errors.New(msg)
	}
	// find original record in foxden metadata database
	spec := make(map[string]any)
	spec["did"] = did
	records := metaDB.Get(
		srvConfig.Config.CHESSMetaData.MongoDB.DBName,
		srvConfig.Config.CHESSMetaData.MongoDB.DBColl,
		spec, 0, 1)
	if len(records) != 1 {
		msg := fmt.Sprintf("unable to find metadata record for did=%s", did)
		return errors.New(msg)
	}
	err := metaDB.InsertRecord(srvConfig.Config.CHESSMetaData.MongoDB.DBName, "archive", records[0])
	return err
}

// helper function to insert data into backend DB
func insertData(sname string, rec map[string]any, attrs, sep, div string, updateRecord bool) (string, error) {
	// load our schema
	if _, err := _smgr.Load(sname); err != nil {
		msg := fmt.Sprintf("unable to load %s error %v", sname, err)
		log.Println("ERROR: ", msg)
		return "", errors.New(msg)
	}

	// check if data satisfies to one of the schema
	if err := validateData(sname, rec); err != nil {
		return "", err
	}
	if _, ok := rec["date"]; !ok {
		rec["date"] = time.Now().Unix()
	}
	rec["schema_file"] = sname
	rec["schema"] = schemaName(sname)
	if link, err := globusLink(rec); err == nil {
		rec["globus_link"] = link
	} else {
		log.Printf("WARNING: unable to create globus link %v", err)
	}
	// add doi attributes
	doiAttributes := []string{"doi", "doi_url", "doi_user", "doi_created_at", "doi_public", "doi_provider", "doi_access_metadata"}
	for _, attr := range doiAttributes {
		if _, ok := rec[attr]; !ok {
			if attr == "doi_public" {
				rec[attr] = false
			} else {
				rec[attr] = ""
			}
		}
	}

	// generate unique id
	didValue, ok := rec["did"]
	did := fmt.Sprintf("%s", didValue)
	if !ok || did == "" {
		// create did out of provided attributes
		did = utils.CreateDID(rec, attrs, sep, div)
		rec["did"] = did
	}
	// for testing purposes with hey we will replace __PLACEHOLDER__ in DID
	if strings.Contains(did, "__PLACEHOLDER__") {
		tstamp := fmt.Sprintf("%d", time.Now().UnixNano())
		did = strings.Replace(did, "__PLACEHOLDER__", tstamp, -1)
	}

	// based on updateRecord decide if we'll insert or update record
	var err error
	if updateRecord {
		// add original record to foxden archive
		err = archiveRecord(rec)
		if err != nil {
			log.Printf("ERROR: unable to archive record for did=%s, error=%v", did, err)
			return did, err
		}
		// add history part of the record
		if user, ok := rec["user"]; ok {
			hrec := HistoryRecord{User: user.(string), Timestamp: time.Now().Unix()}
			var hrecords []HistoryRecord
			if val, ok := rec["history"]; ok {
				hrecords = val.([]HistoryRecord)
				hrecords = append(hrecords, hrec)
				rec["history"] = hrecords
			} else {
				rec["history"] = []HistoryRecord{hrec}
			}
		} else {
			msg := fmt.Sprintf("Metadata record does not contain user key")
			return did, errors.New(msg)
		}
		// add record to metaDB DB
		var records []map[string]any
		records = append(records, rec)
		err = metaDB.Upsert(
			srvConfig.Config.CHESSMetaData.MongoDB.DBName,
			srvConfig.Config.CHESSMetaData.MongoDB.DBColl,
			"did", records)
		if err != nil {
			log.Printf("ERROR: unable to metaDB.Upsert for did=%s, error=%v", did, err)
		}
		return did, err
	}

	// check if did already exist in metaDB
	spec := map[string]any{"did": did}
	records := metaDB.Get(
		srvConfig.Config.CHESSMetaData.MongoDB.DBName,
		srvConfig.Config.CHESSMetaData.MongoDB.DBColl,
		spec, 0, 1)
	if len(records) > 0 {
		msg := fmt.Sprintf("Record with did=%s found in MetaData database %+v", did, records)
		return did, errors.New(msg)
	}
	if Verbose > 0 {
		log.Printf("insert data %+v", rec)
	}

	// insert record to metaDB
	err = metaDB.InsertRecord(
		srvConfig.Config.CHESSMetaData.MongoDB.DBName,
		srvConfig.Config.CHESSMetaData.MongoDB.DBColl,
		rec)
	log.Println("metaDB.InsertRecord", err)

	return did, err
}
