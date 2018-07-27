package encoding

import (
	"bytes"
	"encoding/gob"
	"io"

	jsoniter "github.com/json-iterator/go"
)

//Codec 参考https://github.com/golang/appengine/blob/master/memcache/memcache.go
type Codec struct {
	Marshal   func(interface{}) ([]byte, error)
	Unmarshal func([]byte, interface{}) error
}

var (
	// Gob is a Codec that uses the gob package.
	Gob = Codec{gobMarshal, gobUnmarshal}
	// JSON is a Codec that uses the json package.
	// JSON = Codec{json.Marshal, json.Unmarshal}
	JSON = jsoniter.ConfigCompatibleWithStandardLibrary
)

func gobMarshal(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gobUnmarshal(data []byte, v interface{}) error {
	return gob.NewDecoder(bytes.NewBuffer(data)).Decode(v)
}

func JSONReaderUnmarshal(r io.Reader, d interface{}) (err error) {
	return jsoniter.NewDecoder(r).Decode(d)
}
func JSONWriterMarshal(w io.Writer, d interface{}) (err error) {
	enc := jsoniter.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(d)
}
