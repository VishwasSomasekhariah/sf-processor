//
// Copyright (C) 2020 IBM Corporation.
//
// Authors:
// Frederico Araujo <frederico.araujo@ibm.com>
// Teryl Taylor <terylt@ibm.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
package engine

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/cespare/xxhash"
	"github.com/sysflow-telemetry/sf-apis/go/sfgo"
	"github.com/sysflow-telemetry/sf-apis/go/utils"
	"github.ibm.com/sysflow/goutils/logger"
	"github.ibm.com/sysflow/sf-processor/core/flattener"
)

// FieldMap is a functional type denoting a SysFlow attribute mapper.
type FieldMap func(r *Record) interface{}

// IntFieldMap is a functional type denoting a numerical attribute mapper.
type IntFieldMap func(r *Record) int64

// StrFieldMap is a functional type denoting a string attribute mapper.
type StrFieldMap func(r *Record) string

// FieldMapper is an adapter for SysFlow attribute mappers.
type FieldMapper struct {
	Mappers map[string]FieldMap
}

// Map retrieves a field map based on a SysFlow attribute.
func (m FieldMapper) Map(attr string) FieldMap {
	if mapper, ok := m.Mappers[attr]; ok {
		return mapper
	}
	return func(r *Record) interface{} { return attr }
}

// MapInt retrieves a numerical field map based on a SysFlow attribute.
func (m FieldMapper) MapInt(attr string) IntFieldMap {
	return func(r *Record) int64 {
		if v, ok := m.Map(attr)(r).(int64); ok {
			return v
		} else if v, err := strconv.ParseInt(attr, 10, 64); err == nil {
			return v
		}
		return sfgo.Zeros.Int64
	}
}

// MapStr retrieves a string field map based on a SysFlow attribute.
func (m FieldMapper) MapStr(attr string) StrFieldMap {
	return func(r *Record) string {
		if v, ok := m.Map(attr)(r).(string); ok {
			return m.trimBoundingQuotes(v)
		} else if v, ok := m.Map(attr)(r).(int64); ok {
			return strconv.FormatInt(v, 10)
		} else if v, ok := m.Map(attr)(r).(bool); ok {
			return strconv.FormatBool(v)
		}
		return sfgo.Zeros.String
	}
}

func (m FieldMapper) trimBoundingQuotes(s string) string {
	if len(s) > 0 && (s[0] == '"' || s[0] == '\'') {
		s = s[1:]
	}
	if len(s) > 0 && (s[len(s)-1] == '"' || s[len(s)-1] == '\'') {
		s = s[:len(s)-1]
	}
	return s
}

func getFields() []string {
	keys := make([]string, 0, len(Mapper.Mappers))
	for k := range Mapper.Mappers {
		keys = append(keys, k)
	}
	sort.SliceStable(keys, func(i int, j int) bool {
		ki := len(strings.Split(keys[i], "."))
		kj := len(strings.Split(keys[j], "."))
		if ki == kj {
			return strings.Compare(keys[i], keys[j]) < 0
		}
		return ki < kj
	})
	return keys
}

// Fields defines a sorted array of all field mapper keys.
var Fields = getFields()

// Mapper defines a global attribute mapper instance.
var Mapper = FieldMapper{
	map[string]FieldMap{
		SF_TYPE:                  mapRecType(flattener.SYSFLOW_SRC),
		SF_OPFLAGS:               mapOpFlags(flattener.SYSFLOW_SRC),
		SF_RET:                   mapRet(flattener.SYSFLOW_SRC),
		SF_TS:                    mapInt(flattener.SYSFLOW_SRC, sfgo.TS_INT),
		SF_ENDTS:                 mapEndTs(flattener.SYSFLOW_SRC),
		SF_PROC_OID:              mapOID(flattener.SYSFLOW_SRC, sfgo.PROC_OID_HPID_INT, sfgo.PROC_OID_CREATETS_INT),
		SF_PROC_PID:              mapInt(flattener.SYSFLOW_SRC, sfgo.PROC_OID_HPID_INT),
		SF_PROC_NAME:             mapName(flattener.SYSFLOW_SRC, sfgo.PROC_EXE_STR),
		SF_PROC_EXE:              mapStr(flattener.SYSFLOW_SRC, sfgo.PROC_EXE_STR),
		SF_PROC_ARGS:             mapStr(flattener.SYSFLOW_SRC, sfgo.PROC_EXEARGS_STR),
		SF_PROC_UID:              mapInt(flattener.SYSFLOW_SRC, sfgo.PROC_UID_INT),
		SF_PROC_USER:             mapStr(flattener.SYSFLOW_SRC, sfgo.PROC_USERNAME_STR),
		SF_PROC_TID:              mapInt(flattener.SYSFLOW_SRC, sfgo.TID_INT),
		SF_PROC_GID:              mapInt(flattener.SYSFLOW_SRC, sfgo.PROC_GID_INT),
		SF_PROC_GROUP:            mapStr(flattener.SYSFLOW_SRC, sfgo.PROC_GROUPNAME_STR),
		SF_PROC_CREATETS:         mapInt(flattener.SYSFLOW_SRC, sfgo.PROC_OID_CREATETS_INT),
		SF_PROC_TTY:              mapInt(flattener.SYSFLOW_SRC, sfgo.PROC_TTY_INT),
		SF_PROC_ENTRY:            mapEntry(flattener.SYSFLOW_SRC, sfgo.PROC_ENTRY_INT),
		SF_PROC_CMDLINE:          mapJoin(flattener.SYSFLOW_SRC, sfgo.PROC_EXE_STR, sfgo.PROC_EXEARGS_STR),
		SF_PROC_ANAME:            mapCachedValue(flattener.SYSFLOW_SRC, ProcAName),
		SF_PROC_AEXE:             mapCachedValue(flattener.SYSFLOW_SRC, ProcAExe),
		SF_PROC_ACMDLINE:         mapCachedValue(flattener.SYSFLOW_SRC, ProcACmdLine),
		SF_PROC_APID:             mapCachedValue(flattener.SYSFLOW_SRC, ProcAPID),
		SF_PPROC_OID:             mapOID(flattener.SYSFLOW_SRC, sfgo.PROC_POID_HPID_INT, sfgo.PROC_POID_CREATETS_INT),
		SF_PPROC_PID:             mapInt(flattener.SYSFLOW_SRC, sfgo.PROC_POID_HPID_INT),
		SF_PPROC_NAME:            mapCachedValue(flattener.SYSFLOW_SRC, PProcName),
		SF_PPROC_EXE:             mapCachedValue(flattener.SYSFLOW_SRC, PProcExe),
		SF_PPROC_ARGS:            mapCachedValue(flattener.SYSFLOW_SRC, PProcArgs),
		SF_PPROC_UID:             mapCachedValue(flattener.SYSFLOW_SRC, PProcUID),
		SF_PPROC_USER:            mapCachedValue(flattener.SYSFLOW_SRC, PProcUser),
		SF_PPROC_GID:             mapCachedValue(flattener.SYSFLOW_SRC, PProcGID),
		SF_PPROC_GROUP:           mapCachedValue(flattener.SYSFLOW_SRC, PProcGroup),
		SF_PPROC_CREATETS:        mapInt(flattener.SYSFLOW_SRC, sfgo.PROC_POID_CREATETS_INT),
		SF_PPROC_TTY:             mapCachedValue(flattener.SYSFLOW_SRC, PProcTTY),
		SF_PPROC_ENTRY:           mapCachedValue(flattener.SYSFLOW_SRC, PProcEntry),
		SF_PPROC_CMDLINE:         mapCachedValue(flattener.SYSFLOW_SRC, PProcCmdLine),
		SF_FILE_NAME:             mapName(flattener.SYSFLOW_SRC, sfgo.FILE_PATH_STR),
		SF_FILE_PATH:             mapStr(flattener.SYSFLOW_SRC, sfgo.FILE_PATH_STR),
		SF_FILE_CANONICALPATH:    mapLinkPath(flattener.SYSFLOW_SRC, sfgo.FILE_PATH_STR),
		SF_FILE_OID:              mapOID(flattener.SYSFLOW_SRC, sfgo.FILE_PATH_STR),
		SF_FILE_DIRECTORY:        mapDir(flattener.SYSFLOW_SRC, sfgo.FILE_PATH_STR),
		SF_FILE_NEWNAME:          mapName(flattener.SYSFLOW_SRC, sfgo.SEC_FILE_PATH_STR),
		SF_FILE_NEWPATH:          mapStr(flattener.SYSFLOW_SRC, sfgo.SEC_FILE_PATH_STR),
		SF_FILE_NEWCANONICALPATH: mapLinkPath(flattener.SYSFLOW_SRC, sfgo.SEC_FILE_PATH_STR),
		SF_FILE_NEWOID:           mapOID(flattener.SYSFLOW_SRC, sfgo.SEC_FILE_PATH_STR),
		SF_FILE_NEWDIRECTORY:     mapDir(flattener.SYSFLOW_SRC, sfgo.SEC_FILE_PATH_STR),
		SF_FILE_TYPE:             mapFileType(flattener.SYSFLOW_SRC, sfgo.FILE_RESTYPE_INT),
		SF_FILE_IS_OPEN_WRITE:    mapIsOpenWrite(flattener.SYSFLOW_SRC, sfgo.FL_FILE_OPENFLAGS_INT),
		SF_FILE_IS_OPEN_READ:     mapIsOpenRead(flattener.SYSFLOW_SRC, sfgo.FL_FILE_OPENFLAGS_INT),
		SF_FILE_FD:               mapInt(flattener.SYSFLOW_SRC, sfgo.FL_FILE_FD_INT),
		SF_FILE_OPENFLAGS:        mapOpenFlags(flattener.SYSFLOW_SRC, sfgo.FL_FILE_OPENFLAGS_INT),
		SF_NET_PROTO:             mapInt(flattener.SYSFLOW_SRC, sfgo.FL_NETW_PROTO_INT),
		SF_NET_PROTONAME:         mapProto(flattener.SYSFLOW_SRC, sfgo.FL_NETW_PROTO_INT),
		SF_NET_SPORT:             mapInt(flattener.SYSFLOW_SRC, sfgo.FL_NETW_SPORT_INT),
		SF_NET_DPORT:             mapInt(flattener.SYSFLOW_SRC, sfgo.FL_NETW_DPORT_INT),
		SF_NET_PORT:              mapPort(flattener.SYSFLOW_SRC, sfgo.FL_NETW_SPORT_INT, sfgo.FL_NETW_DPORT_INT),
		SF_NET_SIP:               mapIP(flattener.SYSFLOW_SRC, sfgo.FL_NETW_SIP_INT),
		SF_NET_DIP:               mapIP(flattener.SYSFLOW_SRC, sfgo.FL_NETW_DIP_INT),
		SF_NET_IP:                mapIP(flattener.SYSFLOW_SRC, sfgo.FL_NETW_SIP_INT, sfgo.FL_NETW_DIP_INT),
		SF_FLOW_RBYTES:           mapSum(flattener.SYSFLOW_SRC, sfgo.FL_FILE_NUMRRECVBYTES_INT, sfgo.FL_NETW_NUMRRECVBYTES_INT),
		SF_FLOW_ROPS:             mapSum(flattener.SYSFLOW_SRC, sfgo.FL_FILE_NUMRRECVOPS_INT, sfgo.FL_NETW_NUMRRECVOPS_INT),
		SF_FLOW_WBYTES:           mapSum(flattener.SYSFLOW_SRC, sfgo.FL_FILE_NUMWSENDBYTES_INT, sfgo.FL_NETW_NUMWSENDBYTES_INT),
		SF_FLOW_WOPS:             mapSum(flattener.SYSFLOW_SRC, sfgo.FL_FILE_NUMWSENDOPS_INT, sfgo.FL_NETW_NUMWSENDOPS_INT),
		SF_CONTAINER_ID:          mapStr(flattener.SYSFLOW_SRC, sfgo.CONT_ID_STR),
		SF_CONTAINER_NAME:        mapStr(flattener.SYSFLOW_SRC, sfgo.CONT_NAME_STR),
		SF_CONTAINER_IMAGEID:     mapStr(flattener.SYSFLOW_SRC, sfgo.CONT_IMAGEID_STR),
		SF_CONTAINER_IMAGE:       mapStr(flattener.SYSFLOW_SRC, sfgo.CONT_IMAGE_STR),
		SF_CONTAINER_TYPE:        mapContType(flattener.SYSFLOW_SRC, sfgo.CONT_TYPE_INT),
		SF_CONTAINER_PRIVILEGED:  mapInt(flattener.SYSFLOW_SRC, sfgo.CONT_PRIVILEGED_INT),
		SF_NODE_ID:               mapStr(flattener.SYSFLOW_SRC, sfgo.SFHE_EXPORTER_STR),
		SF_NODE_IP:               mapStr(flattener.SYSFLOW_SRC, sfgo.SFHE_IP_STR),
		SF_SCHEMA_VERSION:        mapInt(flattener.SYSFLOW_SRC, sfgo.SFHE_VERSION_INT),

		//Ext processes
		EXT_PROC_GUID_STR:                mapStr(flattener.PROCESS_SRC, flattener.PROC_GUID_STR),
		EXT_PROC_IMAGE_STR:               mapStr(flattener.PROCESS_SRC, flattener.PROC_IMAGE_STR),
		EXT_PROC_CURR_DIRECTORY_STR:      mapDir(flattener.PROCESS_SRC, flattener.PROC_CURR_DIRECTORY_STR),
		EXT_PROC_LOGON_GUID_STR:          mapStr(flattener.PROCESS_SRC, flattener.PROC_LOGON_GUID_STR),
		EXT_PROC_LOGON_ID_STR:            mapStr(flattener.PROCESS_SRC, flattener.PROC_LOGON_ID_STR),
		EXT_PROC_TERMINAL_SESSION_ID_STR: mapStr(flattener.PROCESS_SRC, flattener.PROC_TERMINAL_SESSION_ID_STR),
		EXT_PROC_INTEGRITY_LEVEL_STR:     mapStr(flattener.PROCESS_SRC, flattener.PROC_INTEGRITY_LEVEL_STR),
		EXT_PROC_SIGNATURE_STR:           mapStr(flattener.PROCESS_SRC, flattener.PROC_SIGNATURE_STR),
		EXT_PROC_SIGNATURE_STATUS_STR:    mapStr(flattener.PROCESS_SRC, flattener.PROC_SIGNATURE_STATUS_STR),
		EXT_PROC_SHA1_HASH_STR:           mapStr(flattener.PROCESS_SRC, flattener.PROC_SHA1_HASH_STR),
		EXT_PROC_MD5_HASH_STR:            mapStr(flattener.PROCESS_SRC, flattener.PROC_MD5_HASH_STR),
		EXT_PROC_SHA256_HASH_STR:         mapStr(flattener.PROCESS_SRC, flattener.PROC_SHA256_HASH_STR),
		EXT_PROC_IMP_HASH_STR:            mapStr(flattener.PROCESS_SRC, flattener.PROC_IMP_HASH_STR),
		EXT_PROC_SIGNED_INT:              mapInt(flattener.PROCESS_SRC, flattener.PROC_SIGNED_INT),

		//Ext files
		EXT_FILE_SIGNATURE_STR:        mapStr(flattener.FILE_SRC, flattener.FILE_SIGNATURE_STR),
		EXT_FILE_SIGNATURE_STATUS_STR: mapStr(flattener.FILE_SRC, flattener.FILE_SIGNATURE_STATUS_STR),
		EXT_FILE_SHA1_HASH_STR:        mapStr(flattener.FILE_SRC, flattener.FILE_SHA1_HASH_STR),
		EXT_FILE_MD5_HASH_STR:         mapStr(flattener.FILE_SRC, flattener.FILE_MD5_HASH_STR),
		EXT_FILE_SHA256_HASH_STR:      mapStr(flattener.FILE_SRC, flattener.FILE_SHA256_HASH_STR),
		EXT_FILE_IMP_HASH_STR:         mapStr(flattener.FILE_SRC, flattener.FILE_IMP_HASH_STR),
		EXT_FILE_SIGNED_INT:           mapInt(flattener.FILE_SRC, flattener.FILE_SIGNED_INT),

		//Ext network
		EXT_NET_SOURCE_HOST_NAME_STR: mapStr(flattener.NETWORK_SRC, flattener.NET_SOURCE_HOST_NAME_STR),
		EXT_NET_SOURCE_PORT_NAME_STR: mapStr(flattener.NETWORK_SRC, flattener.NET_SOURCE_PORT_NAME_STR),
		EXT_NET_DEST_HOST_NAME_STR:   mapStr(flattener.NETWORK_SRC, flattener.NET_DEST_HOST_NAME_STR),
		EXT_NET_DEST_PORT_NAME_STR:   mapStr(flattener.NETWORK_SRC, flattener.NET_DEST_PORT_NAME_STR),

		//Ext target proc
		EXT_TARG_PROC_OID_CREATETS_INT:       mapInt(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_OID_CREATETS_INT),
		EXT_TARG_PROC_OID_HPID_INT:           mapInt(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_OID_HPID_INT),
		EXT_TARG_PROC_TS_INT:                 mapInt(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_TS_INT),
		EXT_TARG_PROC_POID_CREATETS_INT:      mapInt(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_POID_CREATETS_INT),
		EXT_TARG_PROC_POID_HPID_INT:          mapInt(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_POID_HPID_INT),
		EXT_TARG_PROC_EXE_STR:                mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_EXE_STR),
		EXT_TARG_PROC_EXEARGS_STR:            mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_EXEARGS_STR),
		EXT_TARG_PROC_UID_INT:                mapInt(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_UID_INT),
		EXT_TARG_PROC_GID_INT:                mapInt(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_GID_INT),
		EXT_TARG_PROC_USERNAME_STR:           mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_USERNAME_STR),
		EXT_TARG_PROC_GROUPNAME_STR:          mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_GROUPNAME_STR),
		EXT_TARG_PROC_TTY_INT:                mapInt(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_TTY_INT),
		EXT_TARG_PROC_CONTAINERID_STRING_STR: mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_CONTAINERID_STRING_STR),
		EXT_TARG_PROC_ENTRY_INT:              mapEntry(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_ENTRY_INT),

		EXT_TARG_PROC_GUID_STR:                mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_GUID_STR),
		EXT_TARG_PROC_IMAGE_STR:               mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_IMAGE_STR),
		EXT_TARG_PROC_CURR_DIRECTORY_STR:      mapDir(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_CURR_DIRECTORY_STR),
		EXT_TARG_PROC_LOGON_GUID_STR:          mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_LOGON_GUID_STR),
		EXT_TARG_PROC_LOGON_ID_STR:            mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_LOGON_ID_STR),
		EXT_TARG_PROC_TERMINAL_SESSION_ID_STR: mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_TERMINAL_SESSION_ID_STR),
		EXT_TARG_PROC_INTEGRITY_LEVEL_STR:     mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_INTEGRITY_LEVEL_STR),
		EXT_TARG_PROC_SIGNATURE_STR:           mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_SIGNATURE_STR),
		EXT_TARG_PROC_SIGNATURE_STATUS_STR:    mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_SIGNATURE_STATUS_STR),
		EXT_TARG_PROC_SHA1_HASH_STR:           mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_SHA1_HASH_STR),
		EXT_TARG_PROC_MD5_HASH_STR:            mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_MD5_HASH_STR),
		EXT_TARG_PROC_SHA256_HASH_STR:         mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_SHA256_HASH_STR),
		EXT_TARG_PROC_IMP_HASH_STR:            mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_IMP_HASH_STR),
		EXT_TARG_PROC_SIGNED_INT:              mapInt(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_SIGNED_INT),
		EXT_TARG_PROC_START_ADDR_STR:          mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_START_ADDR_STR),
		EXT_TARG_PROC_START_MODULE_STR:        mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_START_MODULE_STR),
		EXT_TARG_PROC_START_FUNCTION_STR:      mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_START_FUNCTION_STR),
		EXT_TARG_PROC_GRANT_ACCESS_STR:        mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_GRANT_ACCESS_STR),
		EXT_TARG_PROC_CALL_TRACE_STR:          mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_CALL_TRACE_STR),
		EXT_TARG_PROC_ACCESS_TYPE_STR:         mapStr(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_ACCESS_TYPE_STR),
		EXT_TARG_PROC_NEW_THREAD_ID_INT:       mapInt(flattener.TARG_PROC_SRC, flattener.EVT_TARG_PROC_NEW_THREAD_ID_INT),
	},
}

func mapStr(src flattener.Source, attr sfgo.Attribute) FieldMap {
	return func(r *Record) interface{} { return r.GetStr(attr, src) }
}

func mapInt(src flattener.Source, attr sfgo.Attribute) FieldMap {
	return func(r *Record) interface{} { return r.GetInt(attr, src) }
}

func mapSum(src flattener.Source, attrs ...sfgo.Attribute) FieldMap {
	return func(r *Record) interface{} {
		var sum int64 = 0
		for _, attr := range attrs {
			sum += r.GetInt(attr, src)
		}
		return sum
	}
}

func mapJoin(src flattener.Source, attrs ...sfgo.Attribute) FieldMap {
	return func(r *Record) interface{} {
		var join string = r.GetStr(attrs[0], src)
		for _, attr := range attrs[1:] {
			join += SPACE + r.GetStr(attr, src)
		}
		return join
	}
}

func mapRecType(src flattener.Source) FieldMap {
	return func(r *Record) interface{} {
		switch r.GetInt(sfgo.SF_REC_TYPE, src) {
		case sfgo.PROC:
			return TyP
		case sfgo.FILE:
			return TyF
		case sfgo.CONT:
			return TyC
		case sfgo.PROC_EVT:
			return TyPE
		case sfgo.FILE_EVT:
			return TyFE
		case sfgo.FILE_FLOW:
			return TyFF
		case sfgo.NET_FLOW:
			return TyNF
		case sfgo.HEADER:
			return TyH
		default:
			return TyUnknow
		}
	}
}

func mapOpFlags(src flattener.Source) FieldMap {
	return func(r *Record) interface{} {
		opflags := r.GetInt(sfgo.EV_PROC_OPFLAGS_INT, src)
		rtype := mapRecType(src)(r).(string)
		return strings.Join(utils.GetOpFlags(int32(opflags), rtype), LISTSEP)
	}
}

func mapRet(src flattener.Source) FieldMap {
	return func(r *Record) interface{} {
		switch r.GetInt(sfgo.SF_REC_TYPE, src) {
		case sfgo.PROC_EVT:
			fallthrough
		case sfgo.FILE_EVT:
			return r.GetInt(sfgo.RET_INT, src)
		default:
			return sfgo.Zeros.Int64
		}
	}
}

func mapEndTs(src flattener.Source) FieldMap {
	return func(r *Record) interface{} {
		switch r.GetInt(sfgo.SF_REC_TYPE, src) {
		case sfgo.FILE_FLOW:
			return r.GetInt(sfgo.FL_FILE_ENDTS_INT, src)
		case sfgo.NET_FLOW:
			return r.GetInt(sfgo.FL_NETW_ENDTS_INT, src)
		default:
			return sfgo.Zeros.Int64
		}
	}
}

func mapEntry(src flattener.Source, attr sfgo.Attribute) FieldMap {
	return func(r *Record) interface{} {
		if r.GetInt(attr, src) == 1 {
			return true
		}
		return false
	}
}

func mapName(src flattener.Source, attr sfgo.Attribute) FieldMap {
	return func(r *Record) interface{} {
		return filepath.Base(r.GetStr(attr, src))
	}
}

func mapDir(src flattener.Source, attr sfgo.Attribute) FieldMap {
	return func(r *Record) interface{} {
		return filepath.Dir(r.GetStr(attr, src))
	}
}

func mapLinkPath(src flattener.Source, attr sfgo.Attribute) FieldMap {
	return func(r *Record) interface{} {
		orig := r.GetStr(attr, src)
		// Possible format: aabbccddeeff0011->aabbccddeeff0011 /path/to/target.file
		var src, dst uint64
		var targetPath string
		if _, err := fmt.Sscanf(orig, "%x->%x %s", &src, &dst, &targetPath); nil == err {
			return targetPath
		}
		return orig
	}
}

func mapFileType(src flattener.Source, attr sfgo.Attribute) FieldMap {
	return func(r *Record) interface{} {
		return utils.GetFileType(r.GetInt(attr, src))
	}
}

func mapIsOpenWrite(src flattener.Source, attr sfgo.Attribute) FieldMap {
	return func(r *Record) interface{} {
		if utils.IsOpenWrite(r.GetInt(attr, src)) {
			return true
		}
		return false
	}
}

func mapIsOpenRead(src flattener.Source, attr sfgo.Attribute) FieldMap {
	return func(r *Record) interface{} {
		if utils.IsOpenRead(r.GetInt(attr, src)) {
			return true
		}
		return false
	}
}

func mapOpenFlags(src flattener.Source, attr sfgo.Attribute) FieldMap {
	return func(r *Record) interface{} {
		return strings.Join(utils.GetOpenFlags(r.GetInt(attr, src)), LISTSEP)
	}
}

func mapProto(src flattener.Source, attr sfgo.Attribute) FieldMap {
	return func(r *Record) interface{} {
		return r.GetInt(attr, src)
	}
}

func mapPort(src flattener.Source, attrs ...sfgo.Attribute) FieldMap {
	return func(r *Record) interface{} {
		var ports = make([]string, 0)
		for _, attr := range attrs {
			ports = append(ports, strconv.FormatInt(r.GetInt(attr, src), 10))
		}
		// logger.Info.Println(ports)
		return strings.Join(ports, LISTSEP)
	}
}

func mapIP(src flattener.Source, attrs ...sfgo.Attribute) FieldMap {
	return func(r *Record) interface{} {
		var ips = make([]string, 0)
		for _, attr := range attrs {
			ips = append(ips, utils.GetIPStr(int32(r.GetInt(attr, src))))
		}
		// logger.Info.Println(ips)
		return strings.Join(ips, LISTSEP)
	}
}

func mapContType(src flattener.Source, attr sfgo.Attribute) FieldMap {
	return func(r *Record) interface{} {
		return utils.GetContType(r.GetInt(attr, src))
	}
}

func mapCachedValue(src flattener.Source, attr RecAttribute) FieldMap {
	return func(r *Record) interface{} {
		oid := sfgo.OID{CreateTS: r.GetInt(sfgo.PROC_OID_CREATETS_INT, src), Hpid: r.GetInt(sfgo.PROC_OID_HPID_INT, src)}
		return r.GetCachedValue(oid, attr)
	}
}

func mapOID(src flattener.Source, attrs ...sfgo.Attribute) FieldMap {
	return func(r *Record) interface{} {
		h := xxhash.New()
		for _, attr := range attrs {
			h.Write([]byte(fmt.Sprintf("%v", r.GetInt(attr, src))))
		}
		return fmt.Sprintf("%x", h.Sum(nil))
	}
}

func mapNa(attr string) FieldMap {
	return func(r *Record) interface{} {
		logger.Warn.Println("Attribute not supported ", attr)
		return sfgo.Zeros.String
	}
}
