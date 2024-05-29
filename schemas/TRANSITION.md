# Transition to FOXDEN
This document provides details of transition to FOXDEN infrastructure.
We will perform the following steps:

1. migrate all existing schemas `ID1A3.json`, `ID3A.json` and `ID4B.json` to new
naming convention
  - in new naming convention we'll use only lower case characters with
  underscore separator, e.g. `BeamSlitVerticalPosition` key will be 
  transformed into `Beam_slit_vertical_position`
2. each schema file may have additional `units` key-value pair providing
specific units for meta-data record, e.g.
```
  {
    "key": "beam_energy",
    "type": "float64",
    "optional": true,
    "multiple": false,
    "section": "Beam",
    "description": "Beam energy",
    "utils": "keV",
    "placeholder": "80.725"
  },
```

In this folder you'll find the following files:
- initial MetaData schema files: `ID1A3.json`, `ID3A.json` and `ID4B.json`
- FOXDEN new (transformed) schema files:
`ID1A3-FOXDEN.json`, `ID3A-FOXDEN.json` and `ID4B-FOXDEN.json`
- initial key files: `ID1A3.keys`, `ID3A.keys` and `ID4B.keys` which contains
list of all schema keys
- FOXDEN new (transformed) key files:
`ID1A3-FOXDEN.keys`, `ID3A-FOXDEN.keys` and `ID4B-FOXDEN.keys`
- and join key files showing the transformation:
`ID1A3.join`, `ID3A.join` and `ID4B.join`

Please review proposed changes before mid July and provide necessary pull
requests to adjust meta-data keys in new schema files along with units section
(if appropriate).
