# MetaData Service
CHESS Meta Data service

#### Example
```
# record.json can be one of CHESS meta-data records

# inject new record
curl -v -X POST -H "Content-type: application/json" \
    -H "Authorization: Bearer $token" \
    -d@./record.json \
    http://localhost:8300

# perform search with pagination
curl -X POST \
    -H "Authorization: bearer $token" \
    -H "Content-type: application/json" \
    -d '{"client":"go-client","service_query":{"query":"{}","spec":null,"sql":"","idx":0,"limit":2}}' \
    http://localhost:8300/search

# retrieve concrete record with did=123456789
curl -H "Accept: application/json" \
    -H "Authorization: bearer $token" \
    http://localhost:8300/123456789
```
