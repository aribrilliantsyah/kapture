package badger

import (
	"encoding/binary"
	"strconv"
	"strings"
	"time"
)

// Key layout (schema 2):
//
//	<date>:<ts>:<namespace>:<workload>:<workload_type>:<pod>:<container>:<level>:<seq>
//
// date is YYYY-MM-DD (UTC) and ts/seq are fixed-width hex, so keys sort
// chronologically across every pod: a range scan returns logs in time order
// and a date is still a cheap DropPrefix. Everything filters need lives in the
// key, so non-matching entries never have their values read.
const (
	keySep        = ":"
	keyParts      = 9
	offsetPrefix  = "_offset:"
	schemaKey     = "_meta:schema"
	schemaVersion = "4"
	tzKey         = "_meta:timezone"
)

// Log keys start with a digit (the date); internal keys start with '_'.
var (
	dataLo = []byte("0")
	dataHi = []byte("_")
)

type keyInfo struct {
	date      string
	ts        int64
	ns        string
	wl        string
	wtype     string
	pod       string
	container string
	level     string
	seq       uint64
}

func encodeKey(k keyInfo) []byte {
	b := make([]byte, 0, 64+len(k.ns)+len(k.wl)+len(k.wtype)+len(k.pod)+len(k.container)+len(k.level))
	b = append(b, k.date...)
	b = append(b, ':')
	b = appendHex16(b, uint64(k.ts))
	for _, s := range []string{k.ns, k.wl, k.wtype, k.pod, k.container, k.level} {
		b = append(b, ':')
		b = append(b, s...)
	}
	b = append(b, ':')
	return appendHex16(b, k.seq)
}

func parseKey(key []byte) (keyInfo, bool) {
	p := strings.SplitN(string(key), keySep, keyParts)
	if len(p) != keyParts || len(p[1]) != 16 {
		return keyInfo{}, false
	}
	ts, err := strconv.ParseUint(p[1], 16, 64)
	if err != nil {
		return keyInfo{}, false
	}
	seq, _ := strconv.ParseUint(p[8], 16, 64)
	return keyInfo{
		date: p[0], ts: int64(ts), ns: p[2], wl: p[3], wtype: p[4],
		pod: p[5], container: p[6], level: p[7], seq: seq,
	}, true
}

// tsBound is the smallest possible key for timestamp ts: every key with that
// timestamp sorts after it, every earlier timestamp sorts before it.
func tsBound(ts int64, loc *time.Location) []byte {
	return appendHex16(append([]byte(dateOf(ts, loc)), ':'), uint64(ts))
}

func appendHex16(dst []byte, v uint64) []byte {
	const digits = "0123456789abcdef"
	for shift := 60; shift >= 0; shift -= 4 {
		dst = append(dst, digits[(v>>uint(shift))&0xf])
	}
	return dst
}

// dateOf is the calendar day of a timestamp in the store's time zone. Days
// only move forward as time does, so keys stay in chronological order.
func dateOf(tsNano int64, loc *time.Location) string {
	return time.Unix(0, tsNano).In(loc).Format("2006-01-02")
}

// clean keeps key segments unambiguous. Kubernetes names never contain ':'.
func clean(s string) string {
	return strings.ReplaceAll(s, keySep, "_")
}

// Values hold only what the key does not: one stream byte plus the message.
func encodeValue(stream, msg string) []byte {
	b := make([]byte, 1+len(msg))
	switch stream {
	case "stdout":
		b[0] = 'o'
	case "stderr":
		b[0] = 'e'
	default:
		b[0] = '-'
	}
	copy(b[1:], msg)
	return b
}

func decodeValue(v []byte) (stream, msg string) {
	if len(v) == 0 {
		return "", ""
	}
	switch v[0] {
	case 'o':
		stream = "stdout"
	case 'e':
		stream = "stderr"
	}
	return stream, string(v[1:])
}

func offsetKey(file string) []byte {
	return []byte(offsetPrefix + file)
}

func encodeOffset(offset int64, inode uint64) []byte {
	b := make([]byte, 16)
	binary.BigEndian.PutUint64(b, uint64(offset))
	binary.BigEndian.PutUint64(b[8:], inode)
	return b
}

func decodeOffset(b []byte) (int64, uint64) {
	if len(b) < 8 {
		return 0, 0
	}
	offset := int64(binary.BigEndian.Uint64(b))
	if len(b) < 16 {
		return offset, 0
	}
	return offset, binary.BigEndian.Uint64(b[8:])
}
