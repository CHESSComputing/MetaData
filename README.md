# MetaData Service
Meta Data service

### APIs

#### public APIs
- `/meta` get all meta data records
- `/meta/:site` get meta data record for a given site

#### Example
```
# get all sites records
curl http://localhost:8300/meta
```

#### protected APIs
- `/meta` post new meta data record
- `/meta/:mid` delete meta data record for a given meta-data ID

#### Example
```
# record.json
{
    "site":"Cornell", 
    "description": "waste minerals", 
    "tags": ["waste", "minerals"],
    "bucket": "waste"
}

# inject new record
curl -v -X POST -H "Content-type: application/json" \
    -H "Authorization: Bearer $token" \
    -d@./record.json \
    http://localhost:8300/meta
```
