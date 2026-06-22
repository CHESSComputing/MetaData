package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	beamlines "github.com/CHESSComputing/golib/beamlines"
	srvConfig "github.com/CHESSComputing/golib/config"
	"github.com/CHESSComputing/golib/globus"
	utils "github.com/CHESSComputing/golib/utils"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// HistoryRecord represents history part of metadata record
type HistoryRecord struct {
	User      string `json:"user"`
	Timestamp int64  `json:"timestamp"`
}

// String provives string representation of history record
func (h *HistoryRecord) String() string {
	if val, err := json.Marshal(h); err == nil {
		return string(val)
	}
	// Avoid recursive call by converting to a non-method type
	type alias HistoryRecord
	return fmt.Sprintf("%v", alias(*h))
}

// helper function to validate input data record against schema
func validateData(sname string, rec map[string]any) error {
	if smgr, ok := _smgr.Map[sname]; ok {
		schema := smgr.Schema
		err := schema.Validate(rec)
		if err != nil {
			return fmt.Errorf("[MetaData.main.validateData] schema.Validate error: %w", err)
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
		msg := fmt.Sprintf("[MetaData.main.globusLink] no data location attributes %v found in meta-data record", locAttrs)
		return "", errors.New(msg)
	}
	pat := "CHESS Raw"
	if srvConfig.Config.Globus.CollectionPath != "" {
		path = srvConfig.Config.Globus.CollectionPath
	}
	gurl, err := globus.ChessGlobusLink(pat, path)
	return gurl, fmt.Errorf("[MetaData.main.globusLink] globus.ChessGlobusLink error: %w", err)
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
	if err != nil {
		return fmt.Errorf("[MetaData.main.updateDoiData] metaDB.Update error: %w", err)
	}
	return nil

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
	if err != nil {
		return fmt.Errorf("[MetaData.main.archiveRecord] metaDB.InsertRecord error: %w", err)
	}
	return nil
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
		return "", fmt.Errorf("[MetaData.main.insertData] validateData error: %w", err)
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
	doiAttributes := []string{"doi", "doi_url", "doi_user", "doi_created_at", "doi_public", "doi_provider", "doi_access_metadata", "doi_foxden_url", "doi_parents_dids"}
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
		if did == "" {
			msg := fmt.Sprintf("fail to create did from attrs=%+v rec=%+v", attrs, rec)
			log.Println("ERROR: ", msg)
			return "", errors.New(msg)
		}
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
			return did, fmt.Errorf("[MetaData.main.insertData] archiveRecord error: %w", err)
		}
		var user string
		if val, ok := rec["user"]; ok {
			user = val.(string)
		} else {
			msg := fmt.Sprintf("Metadata record does not contain user key")
			return did, errors.New(msg)
		}

		// get existing record and extract history records from it
		var hrecords []HistoryRecord
		spec := make(map[string]any)
		spec["did"] = did
		projection := make(map[string]int)
		projection["history"] = 1
		histRecords := metaDB.GetProjection(
			srvConfig.Config.CHESSMetaData.MongoDB.DBName,
			srvConfig.Config.CHESSMetaData.MongoDB.DBColl,
			spec, projection, 0, 1)
		for _, hrec := range histRecords {
			if val, ok := hrec["history"]; ok {
				if hr := getHistoryRecord(val); hr != nil {
					hrecords = append(hrecords, *hr)
				}
			}
		}

		// look-up if provided records has history and update history records accordingly
		if val, ok := rec["history"]; ok {
			var addRecords []HistoryRecord
			for _, hr := range getHistoryRecords(val) {
				for _, hrec := range hrecords {
					if hr != hrec {
						addRecords = append(addRecords, hr)
					}
				}
			}
			hrecords = append(hrecords, addRecords...)
		}

		// add new history record
		hrec := HistoryRecord{User: user, Timestamp: time.Now().Unix()}
		hrecords = append(hrecords, hrec)
		rec["history"] = hrecords

		// add record to metaDB DB
		var records []map[string]any
		records = append(records, rec)
		err = metaDB.Upsert(
			srvConfig.Config.CHESSMetaData.MongoDB.DBName,
			srvConfig.Config.CHESSMetaData.MongoDB.DBColl,
			"did", records)
		if err != nil {
			log.Printf("ERROR: unable to metaDB.Upsert for did=%s, error=%v", did, err)
			return did, fmt.Errorf("[MetaData.main.insertData] metaDB.Upsert error: %w", err)
		}
		return did, nil
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

	if err != nil {
		return did, fmt.Errorf("[MetaData.main.insertData] metaDB.InsertRecord error: %w", err)
	}
	return did, nil
}

// helper function to get history records from given metadata history record part
func getHistoryRecords(val any) []HistoryRecord {
	var hrecords []HistoryRecord
	switch records := val.(type) {
	case []any:
		for _, r := range records {
			if hrec := getHistoryRecord(r); hrec != nil {
				hrecords = append(hrecords, *hrec)
			}
		}
	}
	return hrecords
}

// helper function to deal with any history record structure
func getHistoryRecord(val any) *HistoryRecord {
	log.Printf("getHistoryRecord %+v type %T", val, val)
	switch hr := val.(type) {
	case HistoryRecord:
		return &hr
	case bson.A:
		for _, rec := range hr {
			return decodeHistRecord(rec)
		}
	case map[string]any:
		return decodeHistRecord(hr)
	}
	return nil
}

// helper function to decode history record
func decodeHistRecord(hr any) *HistoryRecord {
	var hrec HistoryRecord
	if data, err := json.Marshal(hr); err == nil {
		if err := json.Unmarshal(data, &hrec); err == nil {
			return &hrec
		} else {
			log.Println("ERROR: unable to unmarshal mongodb record", err)
		}
	} else {
		log.Println("ERROR: unable to marshal mongodb record", err)
	}
	return nil
}

// helper function to validate template record
func validateTmplRecord(rec map[string]any) error {
	val, ok := rec["SchemaName"]
	if !ok {
		return errors.New("provided template record does not have SchemaName key")
	}
	// make local copy of the record without SchemaName for validation purposes
	copyRecord := make(map[string]any)
	for k, v := range rec {
		if k == "SchemaName" {
			continue
		}
		if k == "timestamp" {
			continue
		}
		if k == "did" {
			continue
		}
		copyRecord[k] = v
	}
	sname := fmt.Sprintf("%s", val)
	schemaFile := beamlines.SchemaFileName(sname)
	err := validateData(schemaFile, copyRecord)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "mandatory") {
		// since it is template record we don't need all mandatory keys and will skip this error
		return nil
	}
	return err
}

// TmplRecord handles template records actions
func TmplRecord(rec map[string]any, action string) error {
	collName := srvConfig.Config.CHESSMetaData.DBColl + "_tmpl"
	spec := make(map[string]any)
	if val, ok := rec["btr"]; ok {
		spec["btr"] = val
	} else {
		return errors.New("provided tmp records does not contain btr key")
	}
	if val, ok := rec["sample_name"]; ok {
		spec["sample_name"] = val
	} else {
		return errors.New("provided template record does not contain sample_name key")
	}
	// add timestamp to the record
	rec["timestamp"] = time.Now().Unix()

	// create new did for the record
	did := createDID(rec)
	if did != "" {
		rec["did"] = did
	}

	// first find if such record exists
	nrec := metaDB.Count(
		srvConfig.Config.CHESSMetaData.MongoDB.DBName, collName, spec)
	var err error
	if action == "create" {
		err = metaDB.InsertRecord(
			srvConfig.Config.CHESSMetaData.MongoDB.DBName, collName, rec)
		return err
	}
	if nrec > 0 {
		// if record exists we will update it
		var records []map[string]any
		records = append(records, rec)
		err = metaDB.Upsert(
			srvConfig.Config.CHESSMetaData.MongoDB.DBName, collName, "btr", records)
	} else {
		// if record does not exist we will insert it
		err = metaDB.InsertRecord(
			srvConfig.Config.CHESSMetaData.MongoDB.DBName, collName, rec)
	}
	return err
}

// helper function to create a did of template record
func createDID(rec map[string]any) string {
	if val, ok := rec["did"]; ok {
		return fmt.Sprintf("%v", val)
	}

	// create did for tmpl record if it does not exist
	did := "/tmpl"
	keys := []string{"btr", "cycle", "sample_name", "timestamp"}
	for _, key := range keys {
		if val, ok := rec[key]; ok {
			did = fmt.Sprintf("%s/%s=%v", did, key, val)
		}
	}
	return did
}
