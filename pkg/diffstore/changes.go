package diffstore

import "encoding/json"

type Changes struct {
	Prefix string        `json:"prefix"`
	Set    []interface{} `json:"set,omitempty"`
	Del    []interface{} `json:"del,omitempty"`
}

func (c *Changes) set(v interface{}) {
	c.Set = append(c.Set, v)
}

func (c *Changes) delete(v interface{}) {
	c.Del = append(c.Del, v)
}

func (c Changes) Any() bool {
	return len(c.Set) != 0 || len(c.Del) != 0
}

func (c Changes) String() string {
	ba, _ := json.Marshal(c)
	return string(ba)
}
